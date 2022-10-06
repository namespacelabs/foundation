// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package buildkit

import (
	"context"
	"fmt"
	"io/fs"

	"github.com/moby/buildkit/client/llb"
	dockerfile "github.com/moby/buildkit/frontend/dockerfile/builder"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/build"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/wscontents"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/workspace/tasks"
)

// A Dockerfile build is always relative to the module it lives in. Within that
// module, what's the relative path to the context, and within that context,
// what's the relative path to the Dockerfile.
func DockerfileBuild(rel, dockerfile string, isFocus bool) (build.Spec, error) {
	return dockerfileBuild{rel, dockerfile, isFocus}, nil
}

type dockerfileBuild struct {
	ContextRel string // Relative to the workspace.
	Dockerfile string // Relative to ContextRel.
	IsFocus    bool
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
	generatedRequest := &generateRequest{
		// Setting observeChanges to true will yield a new solve() on changes to the workspace.
		// Also importantly we scope observe changes to ContextRel.
		workspace:  conf.Workspace().VersionedFS(df.ContextRel, df.IsFocus),
		contextRel: df.ContextRel,
		dockerfile: df.Dockerfile,
		conf:       conf,
	}

	return makeImage(env, conf, generatedRequest, []LocalContents{dockerContext(conf, df.ContextRel)}, nil), nil
}

func (df dockerfileBuild) PlatformIndependent() bool { return false }

type generateRequest struct {
	workspace              compute.Computable[wscontents.Versioned] // Used as an input so we trigger new requests on changes to the Dockerfile.
	contextRel, dockerfile string
	conf                   build.Configuration
	compute.LocalScoped[*frontendReq]
}

var _ compute.Computable[*frontendReq] = &generateRequest{}

func (g *generateRequest) Action() *tasks.ActionEvent {
	return tasks.Action("buildkit.make-dockerfile-request").
		Arg("module_name", g.conf.Workspace().ModuleName()).
		Arg("rel", g.contextRel).
		LogLevel(1)
}
func (g *generateRequest) Inputs() *compute.In {
	return compute.Inputs().
		Computable("workspace", g.workspace).
		Str("contextRel", g.contextRel).
		Str("dockerfile", g.dockerfile).
		Indigestible("conf", g.conf)
}
func (g *generateRequest) Output() compute.Output { return compute.Output{NotCacheable: true} }
func (g *generateRequest) Compute(ctx context.Context, deps compute.Resolved) (*frontendReq, error) {
	workspace := compute.MustGetDepValue(deps, g.workspace, "workspace").FS()

	dfcontents, err := fs.ReadFile(workspace, g.dockerfile)
	if err != nil {
		return nil, err
	}

	req := &frontendReq{
		Frontend: "dockerfile.v0",
		FrontendInputs: map[string]llb.State{
			dockerfile.DefaultLocalNameDockerfile: makeDockerfileState(g.conf.SourceLabel(), dfcontents),
			dockerfile.DefaultLocalNameContext:    MakeLocalState(dockerContext(g.conf, g.contextRel)),
		},
	}

	if g.conf.TargetPlatform() != nil {
		req.FrontendOpt = makeDockerOpts([]specs.Platform{*g.conf.TargetPlatform()})
	}

	return req, nil
}

func dockerContext(conf build.Configuration, contextRel string) LocalContents {
	return LocalContents{
		Module:         conf.Workspace(),
		Path:           contextRel,
		ObserveChanges: false, // We don't need to re-observe changes because changes to the workspace will already yield a new frontendReq.
	}
}
