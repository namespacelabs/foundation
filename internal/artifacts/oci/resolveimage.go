// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package oci

import (
	"context"

	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/std/tasks"
)

// Resolves the image tag into a digest. If one is already specified, this is a no-op.
func ResolveDigest(ref string, opts RegistryAccess) NamedImageID {
	return MakeNamedImageID(ref, &resolveDigest{ref: ref, opts: opts})
}

type resolveDigest struct {
	ref  string
	opts RegistryAccess

	compute.LocalScoped[ImageID]
}

func (r *resolveDigest) Inputs() *compute.In {
	return compute.Inputs().Str("ref", r.ref).JSON("opts", r.opts)
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

	desc, err := FetchRemoteDescriptor(ctx, r.ref, r.opts)
	if err != nil {
		return ImageID{}, err
	}

	tasks.Attachments(ctx).AddResult("digest", desc.Digest)

	return imageID.WithDigest(desc.Digest), nil
}
