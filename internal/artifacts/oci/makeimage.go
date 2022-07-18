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

type ResourceDescription[V any] interface {
	Description() string
}

type NamedImageID interface {
	ResourceDescription[ImageID]
	ImageID() compute.Computable[ImageID]
}

type NamedImage interface {
	ResourceDescription[Image]
	Image() compute.Computable[Image]
}

type NamedLayer interface {
	ResourceDescription[Layer]
	Layer() compute.Computable[Layer]
}

func MakeNamedImageID(description string, imageID compute.Computable[ImageID]) NamedImageID {
	return namedImageID{description, imageID}
}

func MakeNamedImage(description string, image compute.Computable[Image]) NamedImage {
	return namedImage{description, image}
}

func MakeNamedLayer(description string, layer compute.Computable[Layer]) NamedLayer {
	return namedLayer{description, layer}
}

func MakeImageFromScratch(description string, layers ...NamedLayer) NamedImage {
	return MakeImage(description, Scratch(), layers...)
}

func MakeImage(description string, base NamedImage, layers ...NamedLayer) NamedImage {
	return &makeImage{base: base, layers: layers}
}

type namedImageID struct {
	description string
	imageID     compute.Computable[ImageID]
}

type namedImage struct {
	description string
	image       compute.Computable[Image]
}

type namedLayer struct {
	description string
	layer       compute.Computable[Layer]
}

func (x namedImageID) ImageID() compute.Computable[ImageID] { return x.imageID }
func (x namedImageID) Description() string                  { return x.description }

func (x namedImage) Image() compute.Computable[Image] { return x.image }
func (x namedImage) Description() string              { return x.description }

func (x namedLayer) Layer() compute.Computable[Layer] { return x.layer }
func (x namedLayer) Description() string              { return x.description }

type makeImage struct {
	base        NamedImage
	layers      []NamedLayer
	description string // Does not affect output.

	compute.LocalScoped[Image]
}

func (al *makeImage) Action() *tasks.ActionEvent {
	var descs []string
	for _, layer := range al.layers {
		if l := layer.Layer(); l != nil {
			descs = append(descs, layer.Description())
		}
	}
	return tasks.Action("oci.make-image").Arg("base", al.base.Description()).Arg("layers", descs)
}

func (al *makeImage) Inputs() *compute.In {
	in := compute.Inputs().Computable("base", al.base.Image())
	for k, layer := range al.layers {
		if l := layer.Layer(); l != nil {
			in = in.Computable(fmt.Sprintf("layer%d", k), l)
		}
	}
	return in
}

func (al *makeImage) Compute(ctx context.Context, deps compute.Resolved) (Image, error) {
	base := compute.MustGetDepValue(deps, al.base.Image(), "base")

	var layers []v1.Layer
	for k, layer := range al.layers {
		l, ok := compute.GetDep(deps, layer.Layer(), fmt.Sprintf("layer%d", k))
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

func (al *makeImage) Image() compute.Computable[Image] { return al }
func (al *makeImage) Description() string              { return al.description }
