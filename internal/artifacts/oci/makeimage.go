// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package oci

import (
	"context"
	"fmt"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func MakeImage(base compute.Computable[Image], layers ...compute.Computable[Layer]) compute.Computable[Image] {
	return &makeImage{base: base, layers: layers}
}

type makeImage struct {
	base   compute.Computable[Image]
	layers []compute.Computable[Layer]

	compute.LocalScoped[Image]
}

func (al *makeImage) Action() *tasks.ActionEvent {
	count := 0
	for _, layer := range al.layers {
		if layer != nil {
			count++
		}
	}
	return tasks.Action("oci.make-image").Arg("base", RefFrom(al.base)).Arg("len(layers)", count)
}

func (al *makeImage) Inputs() *compute.In {
	in := compute.Inputs().Computable("base", al.base)
	for k, layer := range al.layers {
		if layer != nil {
			in = in.Computable(fmt.Sprintf("layer%d", k), layer)
		}
	}
	return in
}

func (al *makeImage) ImageRef() string { return "(new image)" }

func (al *makeImage) Compute(ctx context.Context, deps compute.Resolved) (Image, error) {
	base := compute.GetDepValue(deps, al.base, "base")

	var layers []v1.Layer
	for k, layer := range al.layers {
		l, ok := compute.GetDep(deps, layer, fmt.Sprintf("layer%d", k))
		if ok {
			layers = append(layers, l.Value)
		}
	}

	image, err := mutate.AppendLayers(base, layers...)
	if err != nil {
		return nil, fnerrors.InternalError("failed to create image: %w", err)
	}

	// The Digest() is requested here to guarantee that the image can indeed be created.
	// This will also mark the digest "computed", which is the closest we can get to a
	// sealed result.
	if _, err := image.Digest(); err != nil {
		return nil, fnerrors.InternalError("failed to compute image digest: %w", err)
	}

	return image, nil
}