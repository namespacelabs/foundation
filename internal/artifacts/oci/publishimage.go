// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package oci

import (
	"context"

	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/std/tasks"
)

var ConvertImagesToEstargz = false

func PublishImage(tag compute.Computable[AllocatedName], image NamedImage) NamedImageID {
	return MakeNamedImageID(image.Description(), &publishImage{tag: tag, label: image.Description(), image: AsResolvable(image.Image())})
}

func PublishResolvable(tag compute.Computable[AllocatedName], image compute.Computable[ResolvableImage]) compute.Computable[ImageID] {
	if ConvertImagesToEstargz {
		image = &convertToEstargz{resolvable: image}
	}

	return &publishImage{tag: tag, image: image}
}

type publishImage struct {
	tag   compute.Computable[AllocatedName]
	image compute.Computable[ResolvableImage]
	label string // Does not affect the output.

	compute.LocalScoped[ImageID]
}

func (pi *publishImage) Inputs() *compute.In {
	return compute.Inputs().Computable("tag", pi.tag).Computable("image", pi.image)
}

func (pi *publishImage) Output() compute.Output {
	return compute.Output{NotCacheable: true} // XXX capture more explicitly that there are side-effects.
}

func (pi *publishImage) Action() *tasks.ActionEvent {
	action := tasks.Action("oci.publish-image")
	if pi.label != "" {
		action = action.Arg("image", pi.label)
	}
	return action
}

func (pi *publishImage) Compute(ctx context.Context, deps compute.Resolved) (ImageID, error) {
	tag := compute.MustGetDepValue(deps, pi.tag, "tag")
	tasks.Attachments(ctx).AddResult("ref", tag.ImageRef())
	return compute.MustGetDepValue(deps, pi.image, "image").Push(ctx, tag, true)
}
