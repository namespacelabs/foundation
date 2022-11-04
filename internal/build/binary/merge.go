// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package binary

import (
	"context"

	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/build"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/std/pkggraph"
)

type MergeSpecs struct {
	Specs               []build.Spec
	Descriptions        []string // Same indexing as specs.
	platformIndependent bool
}

func (m MergeSpecs) BuildImage(ctx context.Context, env pkggraph.SealedContext, conf build.Configuration) (compute.Computable[oci.Image], error) {
	images := make([]oci.NamedImage, len(m.Specs))

	for k, spec := range m.Specs {
		xconf := build.CopyConfiguration(conf).WithSourceLabel(m.Descriptions[k])

		// XXX we ignore whether the request is platform-specific.
		image, err := spec.BuildImage(ctx, env, xconf)
		if err != nil {
			return nil, err
		}

		images[k] = oci.MakeNamedImage(m.Descriptions[k], image)
	}

	return oci.MergeImageLayers(images...), nil
}

func (m MergeSpecs) PlatformIndependent() bool { return m.platformIndependent }
