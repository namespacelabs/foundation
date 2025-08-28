// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package oci

import (
	"context"
	"errors"
	"net"
	"syscall"
	"time"

	"github.com/cenkalti/backoff/v4"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/tasks"
)

type TargetRewritter interface {
	CheckRewriteLocalUse(RepositoryWithAccess) *RepositoryWithAccess
}

type ImageSource interface {
	GetSourceLabel() string
	GetSourcePackageRef() *schema.PackageRef
}

var ConvertImagesToEstargz = false

func PublishImage(tag compute.Computable[RepositoryWithParent], image NamedImage) NamedImageID {
	return MakeNamedImageID(image.Description(), &publishImage{tag: tag, label: image.Description(), image: AsResolvable(image.Image())})
}

func PublishResolvable(tag compute.Computable[RepositoryWithParent], image compute.Computable[ResolvableImage], source ImageSource) compute.Computable[ImageID] {
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
	tag         compute.Computable[RepositoryWithParent]
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
	tasks.Attachments(ctx).AddResult("repository", tag.Repository)

	target := tag.RepositoryWithAccess
	if tag.Parent != nil {
		if x, ok := tag.Parent.(TargetRewritter); ok {
			if newTarget := x.CheckRewriteLocalUse(target); newTarget != nil {
				target = *newTarget
			}
		}
	}

	image := compute.MustGetDepValue(deps, pi.image, "image")

	// There are repositories that are not happy with concurrent updates to the
	// same repository. This does not solve concurrent uploads across
	// invocations, but at least we mitigate the problem within a single `ns`
	// invocation.

	return compute.WithLock(ctx, "publish-image:"+target.Repository, func(ctx context.Context) (ImageID, error) {
		var pushedDigest v1.Hash

		if err := backoff.Retry(func() error {
			digest, err := image.Push(ctx, target, true)
			if err != nil {
				return maybeAsPermanent(err)
			}

			pushedDigest = digest
			return nil
		}, &backoff.ExponentialBackOff{
			InitialInterval:     500 * time.Millisecond,
			RandomizationFactor: 0.5,
			Multiplier:          1.5,
			MaxInterval:         5 * time.Second,
			MaxElapsedTime:      2 * time.Minute,
			Clock:               backoff.SystemClock,
		}); err != nil {
			return ImageID{}, err
		}

		// Use the original name, not the rewritten one, for readability purposes.
		return ImageID{Repository: tag.Repository, Digest: pushedDigest.String()}, nil
	})
}

func maybeAsPermanent(err error) error {
	var netErr *net.OpError
	if fnerrors.IsOfKind(err, fnerrors.Kind_INVOCATION) && errors.As(err, &netErr) {
		var errno syscall.Errno
		if errors.As(netErr.Err, &errno) && errno.Is(syscall.EHOSTUNREACH) {
			return err
		}
	}

	return backoff.Permanent(err)
}
