// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package oci

import (
	"context"

	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/tasks"
)

type TargetRewritter interface {
	CheckRewriteLocalUse(TargetRepository) *TargetRepository
}

type ImageSource interface {
	GetSourceLabel() string
	GetSourcePackageRef() *schema.PackageRef
}

var ConvertImagesToEstargz = false

func PublishImage(tag compute.Computable[AllocatedRepository], image NamedImage) NamedImageID {
	return MakeNamedImageID(image.Description(), &publishImage{tag: tag, label: image.Description(), image: AsResolvable(image.Image())})
}

func PublishResolvable(tag compute.Computable[AllocatedRepository], image compute.Computable[ResolvableImage], source ImageSource) compute.Computable[ImageID] {
	if ConvertImagesToEstargz {
		image = &convertToEstargz{resolvable: image}
	}

	sourceLabel := ""
	if source != nil {
		if lbl := source.GetSourceLabel(); lbl != "" {
			sourceLabel = lbl
		} else if ref := source.GetSourcePackageRef(); ref != nil {
			sourceLabel = ref.Canonical()
		}
	}

	return &publishImage{tag: tag, image: image, sourceLabel: sourceLabel}
}

type publishImage struct {
	tag         compute.Computable[AllocatedRepository]
	image       compute.Computable[ResolvableImage]
	label       string // Does not affect the output.
	sourceLabel string // Does not affect the output.

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
	if pi.sourceLabel != "" {
		action = action.Arg("source", pi.sourceLabel)
	}
	return action
}

func (pi *publishImage) Compute(ctx context.Context, deps compute.Resolved) (ImageID, error) {
	tag := compute.MustGetDepValue(deps, pi.tag, "tag")
	tasks.Attachments(ctx).AddResult("ref", tag.ImageRef())

	target := tag.TargetRepository
	if tag.Parent != nil {
		if x, ok := tag.Parent.(TargetRewritter); ok {
			if newTarget := x.CheckRewriteLocalUse(target); newTarget != nil {
				target = *newTarget
			}
		}
	}

	digest, err := compute.MustGetDepValue(deps, pi.image, "image").Push(ctx, target, true)
	if err != nil {
		return ImageID{}, err
	}

	// Use the original name, not the rewritten one, for readability purposes.
	return tag.TargetRepository.WithDigest(digest), nil
}
