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

type NamedImageID struct {
	Description string
	ImageID     compute.Computable[ImageID]
}

type NamedImage struct {
	Description string
	Image       compute.Computable[Image]
}

type NamedLayer struct {
	Description string
	Layer       compute.Computable[Layer]
}

func I(description string, imageID compute.Computable[ImageID]) NamedImageID {
	return NamedImageID{description, imageID}
}

func M(description string, image compute.Computable[Image]) NamedImage {
	return NamedImage{description, image}
}

func L(description string, layer compute.Computable[Layer]) NamedLayer {
	return NamedLayer{description, layer}
}

func MakeImage(base NamedImage, layers ...NamedLayer) compute.Computable[Image] {
	return &makeImage{base: base, layers: layers}
}

type makeImage struct {
	base   NamedImage
	layers []NamedLayer

	compute.LocalScoped[Image]
}

func (al *makeImage) Action() *tasks.ActionEvent {
	var descs []string
	for _, layer := range al.layers {
		if layer.Layer != nil {
			descs = append(descs, layer.Description)
		}
	}
	return tasks.Action("oci.make-image").Arg("base", al.base.Description).Arg("layers", descs)
}

func (al *makeImage) Inputs() *compute.In {
	in := compute.Inputs().Computable("base", al.base.Image)
	for k, layer := range al.layers {
		if layer.Layer != nil {
			in = in.Computable(fmt.Sprintf("layer%d", k), layer.Layer)
		}
	}
	return in
}

func (al *makeImage) Compute(ctx context.Context, deps compute.Resolved) (Image, error) {
	base := compute.MustGetDepValue(deps, al.base.Image, "base")

	var layers []v1.Layer
	for k, layer := range al.layers {
		l, ok := compute.GetDep(deps, layer.Layer, fmt.Sprintf("layer%d", k))
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
