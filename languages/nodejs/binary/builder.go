// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package binary

import (
	"context"

	"namespacelabs.dev/foundation/build"
	"namespacelabs.dev/foundation/build/buildkit"
	"namespacelabs.dev/foundation/engine/compute"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/std/planning"
)

const (
	ForceProd = false
)

func NodejsBuilder(env planning.Context, loc pkggraph.Location, cfg *schema.ImageBuildPlan_NodejsBuild, isFocus bool) (build.Spec, error) {
	return &buildNodeJS{
		loc:        loc.Module.MakeLocation(loc.Rel(cfg.RelPath)),
		isDevBuild: useDevBuild(env.Environment()),
		isFocus:    isFocus,
	}, nil
}

func useDevBuild(env *schema.Environment) bool {
	return !ForceProd && env.Purpose == schema.Environment_DEVELOPMENT
}

type buildNodeJS struct {
	loc        pkggraph.Location
	isDevBuild bool
	isFocus    bool
}

func (buildNodeJS) PlatformIndependent() bool { return false }

func (bnj buildNodeJS) BuildImage(ctx context.Context, env pkggraph.SealedContext, conf build.Configuration) (compute.Computable[oci.Image], error) {
	n := nodeJsBinary{
		nodejsEnv: nodeEnv(env),
	}

	state, local, err := n.LLB(ctx, bnj, conf)
	if err != nil {
		return nil, err
	}

	nodejsImage, err := buildkit.LLBToImage(ctx, env, conf, state, local...)
	if err != nil {
		return nil, err
	}

	return nodejsImage, nil
}

func nodeEnv(env planning.Context) string {
	if env.Environment().GetPurpose() == schema.Environment_PRODUCTION {
		return "production"
	} else {
		return "development"
	}
}
