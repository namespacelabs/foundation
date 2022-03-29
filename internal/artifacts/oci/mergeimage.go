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
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func MergeImageLayers(images ...compute.Computable[Image]) compute.Computable[Image] {
	return &mergeImages{images: images}
}

type mergeImages struct {
	images []compute.Computable[Image]
	compute.LocalScoped[Image]
}

func (al *mergeImages) Action() *tasks.ActionEvent {
	count := 0
	for _, image := range al.images {
		if image != nil {
			count++
		}
	}
	return tasks.Action("oci.merge-images").Arg("len(images)", count)
}

func (al *mergeImages) Inputs() *compute.In {
	in := compute.Inputs()
	for k, image := range al.images {
		in = in.Computable(fmt.Sprintf("image%d", k), image)
	}
	return in
}

func (al *mergeImages) ImageRef() string { return "(new image)" }

func (al *mergeImages) Compute(ctx context.Context, deps compute.Resolved) (Image, error) {
	var layers []v1.Layer
	for k, image := range al.images {
		image, ok := compute.GetDep(deps, image, fmt.Sprintf("image%d", k))
		if !ok {
			continue
		}

		imageLayers, err := image.Value.Layers()
		if err != nil {
			return nil, err
		}
		layers = append(layers, imageLayers...)
	}

	return mutate.AppendLayers(empty.Image, layers...)
}