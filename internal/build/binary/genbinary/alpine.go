// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package genbinary

import (
	"context"
	"strings"

	"github.com/moby/buildkit/client/llb"
	"golang.org/x/exp/slices"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/build"
	"namespacelabs.dev/foundation/internal/build/buildkit"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/dependencies/pins"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/llbutil"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

func BuildAlpine(loc pkggraph.Location, plan *schema.ImageBuildPlan_AlpineBuild) build.Spec {
	return &buildAlpine{loc: loc, plan: plan}
}

type buildAlpine struct {
	loc  pkggraph.Location
	plan *schema.ImageBuildPlan_AlpineBuild
}

func (b *buildAlpine) BuildImage(ctx context.Context, env pkggraph.SealedContext, conf build.Configuration) (compute.Computable[oci.Image], error) {
	var image string
	if b.plan.Version != "" {
		image = "docker.io/library/alpine@" + b.plan.Version
	} else {
		var err error
		image, err = pins.CheckDefault("alpine")
		if err != nil {
			return nil, err
		}
	}

	if conf.TargetPlatform() == nil {
		return nil, fnerrors.InternalError("alpine builds require a platform")
	}

	packages := slices.Clone(b.plan.Package)
	slices.Sort(packages)

	state := llbutil.Image(image, *conf.TargetPlatform()).
		Run(llb.Shlexf("apk add --no-cache %s", strings.Join(packages, " "))).Root()

	def, err := buildkit.MarshalForTarget(ctx, state, conf)
	if err != nil {
		return nil, fnerrors.InternalError("failed to marshal llb plan: %w", err)
	}

	return buildkit.BuildDefinitionToImage(buildkit.DeferClient(env.Configuration(), conf.TargetPlatform()), conf, def), nil
}

func (b *buildAlpine) PlatformIndependent() bool { return false }

func (b *buildAlpine) Description() string { return "makeAlpine" }
