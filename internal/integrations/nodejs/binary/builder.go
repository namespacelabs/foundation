// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package binary

import (
	"context"

	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/build"
	"namespacelabs.dev/foundation/internal/build/assets"
	"namespacelabs.dev/foundation/internal/build/buildkit"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/integrations/opaque"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/pkggraph"
)

func NodejsBuilder(env cfg.Context, loc pkggraph.Location, config *schema.NodejsBuild, assets assets.AvailableBuildAssets, isFocus bool) (build.Spec, error) {
	relPath := config.Pkg
	if relPath == "" {
		relPath = "."
	}

	return &buildNodeJS{
		loc:     loc.Module.MakeLocation(loc.Rel(relPath)),
		config:  config,
		assets:  assets,
		env:     env.Environment(),
		isFocus: isFocus,
	}, nil
}

type buildNodeJS struct {
	loc     pkggraph.Location
	config  *schema.NodejsBuild
	assets  assets.AvailableBuildAssets
	env     *schema.Environment
	isFocus bool
}

func (bnj buildNodeJS) PlatformIndependent() bool {
	// Heuristics: prod builds with an output dir are for Web, thus platform-independent.
	// This is only important for the DevUI prebuilt as for the regular Prod builds the result
	// image contains "nginx" which is platform-dependent.
	return !opaque.UseDevBuild(bnj.env) && bnj.config.Prod.BuildOutDir != ""
}

func (bnj buildNodeJS) BuildImage(ctx context.Context, env pkggraph.SealedContext, conf build.Configuration) (compute.Computable[oci.Image], error) {
	n := nodeJsBinary{
		nodejsEnv: NodeEnv(env.Environment()),
	}

	state, local, err := n.LLB(ctx, env.Configuration(), bnj, conf)
	if err != nil {
		return nil, err
	}

	return buildkit.BuildImage(ctx, buildkit.DeferClient(env.Configuration(), conf.TargetPlatform()), conf, state, local...)
}

func NodeEnv(env *schema.Environment) string {
	if opaque.UseDevBuild(env) {
		return "development"
	} else {
		return "production"
	}
}
