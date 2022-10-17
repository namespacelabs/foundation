// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package oci

import (
	"context"
	"fmt"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/std/tasks"
)

func MergeImageLayers(images ...NamedImage) compute.Computable[Image] {
	return &mergeImages{images: images}
}

type mergeImages struct {
	images []NamedImage
	compute.LocalScoped[Image]
}

func (al *mergeImages) Action() *tasks.ActionEvent {
	var names []string
	for _, image := range al.images {
		if image.Image() == nil {
			continue
		}
		names = append(names, image.Description())
	}
	return tasks.Action("oci.merge-images").Arg("images", names)
}

func (al *mergeImages) Inputs() *compute.In {
	in := compute.Inputs()
	for k, image := range al.images {
		if image.Image() == nil {
			continue
		}
		in = in.Computable(fmt.Sprintf("image%d", k), image.Image())
	}
	return in
}

func (al *mergeImages) Compute(ctx context.Context, deps compute.Resolved) (Image, error) {
	var layers []v1.Layer
	var digests []string
	for k, image := range al.images {
		if image.Image() == nil {
			continue
		}

		image, ok := compute.GetDep(deps, image.Image(), fmt.Sprintf("image%d", k))
		if !ok {
			continue
		}

		digests = append(digests, image.Digest.String())

		imageLayers, err := image.Value.Layers()
		if err != nil {
			return nil, err
		}

		layers = append(layers, imageLayers...)
	}

	tasks.Attachments(ctx).AddResult("digests", digests)

	return mutate.AppendLayers(empty.Image, layers...)
}
