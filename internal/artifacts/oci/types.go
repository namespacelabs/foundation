// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package oci

import (
	"context"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fntypes"
	"namespacelabs.dev/foundation/workspace/cache"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/devhost"
)

type Layer v1.Layer
type Image v1.Image
type ImageIndex v1.ImageIndex

var defaultPlatform = v1.Platform{
	Architecture: "amd64",
	OS:           "linux",
}

type ResolvableImage interface {
	Digest() (fntypes.Digest, error)
	Image() (Image, error)
	ImageIndex() (ImageIndex, error)

	push(context.Context, AllocatedName) (ImageID, error)
	cache(context.Context, cache.Cache) (fntypes.Digest, error)
}

type imageFetchFunc func(v1.Hash) (Image, error)

func WrapImage(c compute.Computable[Image]) compute.Computable[ResolvableImage] {
	return compute.Transform(c, func(ctx context.Context, img Image) (ResolvableImage, error) {
		return rawImage{img}, nil
	})
}

type rawImage struct {
	image v1.Image
}

func (raw rawImage) Digest() (fntypes.Digest, error) {
	h, err := raw.image.Digest()
	return fntypes.Digest(h), err
}

func (raw rawImage) Image() (Image, error) {
	return raw.image, nil
}

func (raw rawImage) ImageIndex() (ImageIndex, error) {
	return nil, fnerrors.InternalError("expected an image index, saw an image")
}

func (raw rawImage) push(ctx context.Context, tag AllocatedName) (ImageID, error) {
	return pushImage(ctx, tag, raw.image)
}

func (raw rawImage) cache(ctx context.Context, c cache.Cache) (fntypes.Digest, error) {
	return imageCacheable{}.Cache(ctx, c, raw.image)
}

type rawImageIndex struct {
	index v1.ImageIndex
}

func (raw rawImageIndex) Digest() (fntypes.Digest, error) {
	h, err := raw.index.Digest()
	return fntypes.Digest(h), err
}

func (raw rawImageIndex) Image() (Image, error) {
	return nil, fnerrors.InternalError("expected an image, saw an image index")
}

func (raw rawImageIndex) ImageIndex() (ImageIndex, error) {
	return raw.index, nil
}

func (raw rawImageIndex) push(ctx context.Context, tag AllocatedName) (ImageID, error) {
	digest, err := raw.index.Digest()
	if err != nil {
		return ImageID{}, err
	}

	if err := pushImageIndex(ctx, tag, raw.index); err != nil {
		return ImageID{}, err
	}

	return tag.WithDigest(digest), nil
}

func (raw rawImageIndex) cache(ctx context.Context, c cache.Cache) (fntypes.Digest, error) {
	return writeImageIndex(ctx, c, raw.index)
}

func imageForPlatform(manifest *v1.IndexManifest, p *specs.Platform, fetch imageFetchFunc) (Image, error) {
	if p == nil {
		return nil, fnerrors.InternalError("failed to select image by platform, no platform specified")
	}

	requested := toV1Plat(p)
	for _, d := range manifest.Manifests {
		plat := defaultPlatform
		if d.Platform != nil {
			// See above, if no platform is specified assume it was the default one.
			plat = *d.Platform
		}

		if platformMatches(&plat, requested) {
			return fetch(d.Digest)
		}
	}

	return nil, fnerrors.BadInputError("no image matches requested platform %q", devhost.FormatPlatform(*p))
}