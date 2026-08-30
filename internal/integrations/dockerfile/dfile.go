// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package dockerfile

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/frontend/dockerfile/dockerignore"
	"github.com/moby/buildkit/frontend/dockerui"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"golang.org/x/exp/maps"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/build"
	"namespacelabs.dev/foundation/internal/build/buildkit"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/parsing/platform"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/std/tasks"
)

// A Dockerfile build is always relative to the module it lives in. Within that
// module, what's the relative path to the context, and within that context,
// what's the relative path to the Dockerfile.
func Build(rel string, d *schema.ImageBuildPlan_DockerBuild) (build.Spec, error) {
	return dockerfileBuild{filepath.Join(rel, d.ContextDir), d}, nil
}

type dockerfileBuild struct {
	contextRel string                             // Relative to the workspace.
	plan       *schema.ImageBuildPlan_DockerBuild // Dockerfile is relative to ContextRel.
}

var _ build.Spec = dockerfileBuild{}

func makeDockerfileState(sourceLabel string, contents []byte) llb.State {
	return llb.Scratch().
		File(llb.Mkfile("/Dockerfile", 0644,
			contents,
			llb.WithCreatedTime(build.FixedPoint)),
			llb.WithCustomName(fmt.Sprintf("Dockerfile (%s)", sourceLabel)))
}

func (df dockerfileBuild) BuildImage(ctx context.Context, env pkggraph.SealedContext, conf build.Configuration) (compute.Computable[oci.Image], error) {
	// There's a compromise here: we go through a non-snapshot to fetch
	// .dockerignore, to avoid creating two snapshots.
	dfignore, err := fs.ReadFile(conf.Workspace().ReadOnlyFS(df.contextRel), ".dockerignore")
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, fnerrors.InternalError("failed to check if a .dockerignore exists: %w", err)
		}
	}

	excludes, err := dockerignore.ReadAll(bytes.NewReader(dfignore))
	if err != nil {
		return nil, fnerrors.Newf("failed to parse dockerignore: %w", err)
	}

	dfcontents, err := fs.ReadFile(conf.Workspace().ReadOnlyFS(df.contextRel), df.plan.Dockerfile)
	if err != nil {
		return nil, err
	}

	generatedRequest := &generateRequest{
		contextRel: df.contextRel,
		dockerfile: string(dfcontents),
		conf:       conf,
		plan:       df.plan,
		excludes:   excludes,
	}

	return buildkit.MakeImage(
		buildkit.DeferClient(env.Configuration(), conf.TargetPlatform()),
		conf,
		generatedRequest,
		[]buildkit.LocalContents{
			dockerContext(conf, df.contextRel, excludes),
		}, conf.PublishName()), nil
}

func (df dockerfileBuild) PlatformIndependent() bool { return false }

func (df dockerfileBuild) Description() string {
	return fmt.Sprintf("fromDockerfile(%s)", filepath.Join(df.contextRel, df.plan.Dockerfile))
}

type generateRequest struct {
	contextRel, dockerfile string
	conf                   build.Configuration
	excludes               []string
	plan                   *schema.ImageBuildPlan_DockerBuild
	compute.LocalScoped[*buildkit.FrontendRequest]
}

var _ compute.Computable[*buildkit.FrontendRequest] = &generateRequest{}

func (g *generateRequest) Action() *tasks.ActionEvent {
	return tasks.Action("buildkit.make-dockerfile-request").
		Arg("module_name", g.conf.Workspace().ModuleName()).
		Arg("rel", g.contextRel).
		LogLevel(1)
}
func (g *generateRequest) Inputs() *compute.In {
	return compute.Inputs().
		Str("contextRel", g.contextRel).
		Str("dockerfile", g.dockerfile).
		JSON("plan", g.plan).
		Indigestible("conf", g.conf)
}
func (g *generateRequest) Output() compute.Output { return compute.Output{NotCacheable: true} }
func (g *generateRequest) Compute(ctx context.Context, deps compute.Resolved) (*buildkit.FrontendRequest, error) {
	req := &buildkit.FrontendRequest{
		Frontend: "dockerfile.v0",
		FrontendInputs: map[string]llb.State{
			dockerui.DefaultLocalNameDockerfile: makeDockerfileState(g.conf.SourceLabel(), []byte(g.dockerfile)),
			dockerui.DefaultLocalNameContext:    buildkit.MakeLocalState(dockerContext(g.conf, g.contextRel, g.excludes)),
		},
	}

	if g.conf.TargetPlatform() != nil {
		req.FrontendAttrs = makeDockerOpts([]specs.Platform{*g.conf.TargetPlatform()})
	}

	if len(g.plan.Attrs) > 0 {
		req.FrontendAttrs = mergeMaps(req.FrontendAttrs, g.plan.Attrs)
	}

	if g.conf.TargetPlatform() != nil {
		p := platform.FormatPlatform(*g.conf.TargetPlatform())

		for _, x := range g.plan.AttrsByPlatform {
			if x.Platform == p {
				req.FrontendAttrs = mergeMaps(req.FrontendAttrs, x.Attrs)
			}
		}
	}

	return req, nil
}

func mergeMaps(target map[string]string, src map[string]string) map[string]string {
	if target == nil {
		return maps.Clone(src)
	}

	for k, v := range src {
		target[k] = v
	}

	return target
}

func dockerContext(conf build.Configuration, contextRel string, excludes []string) buildkit.LocalContents {
	return buildkit.LocalContents{
		Module:          conf.Workspace(),
		Path:            contextRel,
		ExcludePatterns: excludes,
	}
}

func makeDockerOpts(platforms []specs.Platform) map[string]string {
	return map[string]string{
		"platform": strings.Join(platform.FormatPlatforms(platforms), ","),
	}
}
