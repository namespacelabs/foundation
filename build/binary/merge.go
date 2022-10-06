// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package binary

import (
	"context"
	"fmt"

	"namespacelabs.dev/foundation/build"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/std/pkggraph"
)

type mergeSpecs struct {
	specs               []build.Spec
	platformIndependent bool
}

func (m mergeSpecs) BuildImage(ctx context.Context, env pkggraph.SealedContext, conf build.Configuration) (compute.Computable[oci.Image], error) {
	images := make([]oci.NamedImage, len(m.specs))

	for k, spec := range m.specs {
		// XXX we ignore whether the request is platform-specific.
		image, err := spec.BuildImage(ctx, env, conf)
		if err != nil {
			return nil, err
		}

		images[k] = oci.MakeNamedImage(
			fmt.Sprintf("plan#%d", k), // XXX propagate better names.
			image,
		)
	}

	return oci.MergeImageLayers(images...), nil
}

func (m mergeSpecs) PlatformIndependent() bool { return m.platformIndependent }
