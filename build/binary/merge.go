// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package binary

import (
	"context"

	"namespacelabs.dev/foundation/build"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/workspace/compute"
)

type mergeSpecs struct {
	specs               []build.Spec
	platformIndependent bool
}

func (m mergeSpecs) BuildImage(ctx context.Context, env ops.Environment, conf build.Configuration) (compute.Computable[oci.Image], error) {
	images := make([]compute.Computable[oci.Image], len(m.specs))

	for k, spec := range m.specs {
		// XXX we ignore whether the request is platform-specific.
		var err error
		images[k], err = spec.BuildImage(ctx, env, conf)
		if err != nil {
			return nil, err
		}
	}

	return oci.MergeImageLayers(images...), nil
}

func (m mergeSpecs) PlatformIndependent() bool { return m.platformIndependent }
