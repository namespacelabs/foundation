// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package web

import (
	"context"

	"namespacelabs.dev/foundation/build"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/std/planning"
	"namespacelabs.dev/foundation/workspace/compute"
)

func WebBuilder(loc pkggraph.Location) build.Spec {
	return staticBuild{Location: loc}
}

type staticBuild struct {
	Location pkggraph.Location
}

func (w staticBuild) BuildImage(ctx context.Context, env planning.Context, conf build.Configuration) (compute.Computable[oci.Image], error) {
	img, err := ViteProductionBuild(ctx, w.Location, env, conf.SourceLabel(), ".", "/", nil, generateProdViteConfig())
	if err != nil {
		return nil, err
	}

	return img.Image(), nil
}

func (w staticBuild) PlatformIndependent() bool { return true }
