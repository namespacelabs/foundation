// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package docker

import (
	"context"

	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func PublishImage(tag compute.Computable[oci.AllocatedName], image compute.Computable[oci.ResolvableImage]) compute.Computable[oci.ImageID] {
	return &publishImage{tag: tag, image: image}
}

type publishImage struct {
	tag   compute.Computable[oci.AllocatedName]
	image compute.Computable[oci.ResolvableImage]

	compute.LocalScoped[oci.ImageID]
}

func (pi *publishImage) Inputs() *compute.In {
	return compute.Inputs().Computable("tag", pi.tag).Computable("image", pi.image)
}

func (pi *publishImage) Output() compute.Output {
	return compute.Output{NotCacheable: true} // XXX capture more explicitly that there are side-effects.
}

func (pi *publishImage) Action() *tasks.ActionEvent {
	return tasks.Action("docker.publish")
}

// Implements `HasImageRef`.
func (pi *publishImage) ImageRef() string {
	return oci.RefFrom(pi.tag)
}

func (pi *publishImage) Compute(ctx context.Context, deps compute.Resolved) (oci.ImageID, error) {
	tag := compute.MustGetDepValue(deps, pi.tag, "tag")
	resolvable := compute.MustGetDepValue(deps, pi.image, "image")

	tasks.Attachments(ctx).AddResult("tag", tag.ImageRef())

	ref, err := oci.ParseTag(tag)
	if err != nil {
		return oci.ImageID{}, err
	}

	img, err := resolvable.Image()
	if err != nil {
		return oci.ImageID{}, fnerrors.InternalError("docker: %w", err)
	}

	err = WriteImage(ctx, img, ref, true)
	return tag.ImageID, err
}
