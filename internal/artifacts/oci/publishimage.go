// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package oci

import (
	"context"

	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func PublishImage(tag compute.Computable[AllocatedName], image compute.Computable[Image]) compute.Computable[ImageID] {
	return &publishImage{tag: tag, image: AsResolvable(image)}
}

func PublishResolvable(tag compute.Computable[AllocatedName], image compute.Computable[ResolvableImage]) compute.Computable[ImageID] {
	return &publishImage{tag: tag, image: image}
}

type publishImage struct {
	tag   compute.Computable[AllocatedName]
	image compute.Computable[ResolvableImage]

	compute.LocalScoped[ImageID]
}

func (pi *publishImage) Inputs() *compute.In {
	return compute.Inputs().Computable("tag", pi.tag).Computable("image", pi.image)
}

func (pi *publishImage) Output() compute.Output {
	return compute.Output{NotCacheable: true} // XXX capture more explicitly that there are side-effects.
}

func (pi *publishImage) Action() *tasks.ActionEvent {
	return tasks.Action("oci.publish-image")
}

// Implements `HasImageRef`.
func (pi *publishImage) ImageRef() string {
	return RefFrom(pi.tag)
}

func (pi *publishImage) Compute(ctx context.Context, deps compute.Resolved) (ImageID, error) {
	tag := compute.MustGetDepValue(deps, pi.tag, "tag")
	tasks.Attachments(ctx).AddResult("tag", tag.ImageRef())
	return compute.MustGetDepValue(deps, pi.image, "image").Push(ctx, tag)
}
