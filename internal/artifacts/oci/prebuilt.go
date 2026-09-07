// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package oci

import (
	"context"

	"github.com/google/go-containerregistry/pkg/v1/types"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/std/tasks"
)

func Prebuilt(imgid ImageID, opts RegistryAccess) compute.Computable[ResolvableImage] {
	return &fetchPrebuilt{imgid: imgid, opts: opts}
}

type fetchPrebuilt struct {
	imgid ImageID
	opts  RegistryAccess // Does not affect output.

	compute.DoScoped[ResolvableImage]
}

func (f *fetchPrebuilt) Action() *tasks.ActionEvent {
	return tasks.Action("image.fetch").Arg("ref", f.imgid.RepoAndDigest())
}

func (f *fetchPrebuilt) Inputs() *compute.In {
	return compute.Inputs().JSON("imgid", f.imgid)
}

func (f *fetchPrebuilt) Compute(ctx context.Context, _ compute.Resolved) (ResolvableImage, error) {
	descriptor, err := FetchRemoteDescriptor(ctx, f.imgid.RepoAndDigest(), f.opts)
	if err != nil {
		return nil, err
	}

	switch {
	case isIndexMediaType(types.MediaType(descriptor.MediaType)):
		idx, err := descriptor.ImageIndex()
		if err != nil {
			return nil, err
		}

		return rawImageIndex{idx}, nil

	case isImageMediaType(types.MediaType(descriptor.MediaType)):
		img, err := FetchRemoteImage(ctx, f.imgid, f.opts)
		if err != nil {
			return nil, err
		}

		return RawAsResolvable(img), nil
	}

	return nil, fnerrors.InternalError("unknown media type: %v", descriptor.MediaType)
}
