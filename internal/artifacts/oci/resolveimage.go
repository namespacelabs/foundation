// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package oci

import (
	"context"

	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/tasks"
)

// Resolves the image tag into a digest. If one is already specified, this is a no-op.
func ResolveDigest(ref string) compute.Computable[ImageID] {
	return &resolveDigest{ref: ref}
}

type resolveDigest struct {
	ref string

	compute.LocalScoped[ImageID]
}

func (r *resolveDigest) Inputs() *compute.In {
	return compute.Inputs().Str("ref", r.ref)
}

func (r *resolveDigest) ImageRef() string {
	return r.ref
}

func (r *resolveDigest) Action() *tasks.ActionEvent {
	return tasks.Action("oci.resolve-image-digest").Arg("ref", r.ref)
}

func (r *resolveDigest) Compute(ctx context.Context, _ compute.Resolved) (ImageID, error) {
	imageID, err := ParseImageID(r.ref)
	if err != nil {
		return ImageID{}, err
	}

	if imageID.Digest != "" {
		return imageID, nil
	}

	desc, err := fetchRemoteDescriptor(ctx, r.ref)
	if err != nil {
		return ImageID{}, err
	}

	tasks.Attachments(ctx).AddResult("digest", desc.Digest)

	return imageID.WithDigest(desc.Digest), nil
}
