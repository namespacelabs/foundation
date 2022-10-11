// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package genbinary

import (
	"context"
	"strings"

	"github.com/moby/buildkit/client/llb"
	"namespacelabs.dev/foundation/build"
	"namespacelabs.dev/foundation/build/buildkit"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/llbutil"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/workspace/pins"
)

func BuildAlpine(loc pkggraph.Location, plan *schema.ImageBuildPlan_AlpineBuild) build.Spec {
	return &buildAlpine{loc: loc, plan: plan}
}

type buildAlpine struct {
	loc  pkggraph.Location
	plan *schema.ImageBuildPlan_AlpineBuild
}

func (b *buildAlpine) BuildImage(ctx context.Context, env pkggraph.SealedContext, conf build.Configuration) (compute.Computable[oci.Image], error) {
	image, err := pins.CheckDefault("alpine")
	if err != nil {
		return nil, err
	}

	if conf.TargetPlatform() == nil {
		return nil, fnerrors.InternalError("alpine builds require a platform")
	}

	state := llbutil.Image(image, *conf.TargetPlatform()).
		Run(llb.Shlexf("apk add --no-cache %s", strings.Join(b.plan.Package, " ")))

	def, err := state.Marshal(ctx)
	if err != nil {
		return nil, fnerrors.InternalError("failed to marshal llb plan: %w", err)
	}

	return buildkit.BuildDefinitionToImage(env, conf, def), nil
}

func (b *buildAlpine) PlatformIndependent() bool { return false }
