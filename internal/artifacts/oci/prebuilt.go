// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package oci

import (
	"context"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/compute/cache"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/std/tasks"
)

func Prebuilt(imgid ImageID, opts ResolveOpts) compute.Computable[ResolvableImage] {
	return &fetchPrebuilt{imgid: imgid, opts: opts}
}

type fetchPrebuilt struct {
	imgid ImageID
	opts  ResolveOpts // Does not affect output.

	compute.DoScoped[ResolvableImage]
}

func (f *fetchPrebuilt) Action() *tasks.ActionEvent {
	return tasks.Action("image.fetch").Arg("ref", f.imgid.RepoAndDigest())
}

func (f *fetchPrebuilt) Inputs() *compute.In {
	return compute.Inputs().JSON("imgid", f.imgid)
}

func (f *fetchPrebuilt) Compute(ctx context.Context, _ compute.Resolved) (ResolvableImage, error) {
	descriptor, err := fetchRemoteDescriptor(ctx, f.imgid.RepoAndDigest(), f.opts)
	if err != nil {
		return nil, err
	}

	switch {
	case isIndexMediaType(types.MediaType(descriptor.MediaType)):
		idx, err := descriptor.ImageIndex()
		if err != nil {
			return nil, err
		}

		c := compute.Cache(ctx)

		if cache.IsDisabled(c) {
			return rawImageIndex{idx}, nil
		}

		d, err := writeImageIndex(ctx, c, idx)
		if err != nil {
			return nil, err
		}

		return loadCachedResolvable(ctx, c, v1.Hash(d))

	case isImageMediaType(types.MediaType(descriptor.MediaType)):
		img, err := cacheAndReturn(ctx, f.imgid, f.opts)
		if err != nil {
			return nil, err
		}

		return RawAsResolvable(img), nil
	}

	return nil, fnerrors.InternalError("unknown media type: %v", descriptor.MediaType)
}
