// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package buildkit

import (
	"context"
	"fmt"

	"github.com/moby/buildkit/client/llb"
	dockerfile "github.com/moby/buildkit/frontend/dockerfile/builder"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/build"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/workspace/compute"
)

func DockerfileBuild(contents LocalContents, dockerFile []byte) (build.Spec, error) {
	if len(dockerFile) == 0 {
		return nil, fnerrors.InternalError("dockerfile is empty or nont specified")
	}

	return dockerfileBuild{contents, dockerFile}, nil
}

type dockerfileBuild struct {
	Contents   LocalContents
	Dockerfile []byte
}

var _ build.Spec = dockerfileBuild{}

func makeDockerfileState(sourceLabel string, df dockerfileBuild) llb.State {
	return llb.Scratch().
		File(llb.Mkfile("/Dockerfile", 0644,
			df.Dockerfile,
			llb.WithCreatedTime(build.FixedPoint)),
			llb.WithCustomName(fmt.Sprintf("Dockerfile (%s)", sourceLabel)))
}

func (df dockerfileBuild) BuildImage(ctx context.Context, env ops.Environment, conf build.Configuration) (compute.Computable[oci.Image], error) {
	req := &frontendReq{
		Frontend: "dockerfile.v0",
		FrontendInputs: map[string]llb.State{
			dockerfile.DefaultLocalNameDockerfile: makeDockerfileState(conf.SourceLabel(), df),
			dockerfile.DefaultLocalNameContext:    MakeLocalState(df.Contents),
		},
	}

	if conf.TargetPlatform() != nil {
		req.FrontendOpt = makeDockerOpts([]specs.Platform{*conf.TargetPlatform()})
	}

	return makeImage(env, conf, req, []LocalContents{df.Contents}, nil), nil
}

func (df dockerfileBuild) PlatformIndependent() bool { return false }
