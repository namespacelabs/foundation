// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package oci

import (
	"context"
	"fmt"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/types"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/parsing/devhost"
	"namespacelabs.dev/foundation/internal/uniquestrings"
	"namespacelabs.dev/foundation/std/tasks"
)

type ImageWithPlatform struct {
	Image    NamedImage
	Platform specs.Platform
}

func MakeImageIndex(images ...ImageWithPlatform) compute.Computable[ResolvableImage] {
	return &makeImageIndex{images: images}
}

type makeImageIndex struct {
	images []ImageWithPlatform

	compute.LocalScoped[ResolvableImage]
}

func (al *makeImageIndex) Inputs() *compute.In {
	var platforms []string
	in := compute.Inputs()
	for k, d := range al.images {
		in = in.Computable(fmt.Sprintf("image%d", k), d.Image.Image())
		platforms = append(platforms, devhost.FormatPlatform(d.Platform))
	}
	return in.Strs("platforms", platforms)
}

func (al *makeImageIndex) Action() *tasks.ActionEvent {
	var u uniquestrings.List
	var platforms []string
	for _, d := range al.images {
		u.Add(d.Image.Description())
		platforms = append(platforms, devhost.FormatPlatform(d.Platform))
	}
	return tasks.Action("oci.make-image-index").Arg("refs", u.Strings()).Arg("platforms", platforms)
}

func (al *makeImageIndex) Compute(ctx context.Context, deps compute.Resolved) (ResolvableImage, error) {
	var adds []mutate.IndexAddendum
	for k, d := range al.images {
		image := compute.MustGetDepValue(deps, d.Image.Image(), fmt.Sprintf("image%d", k))

		digest, err := image.Digest()
		if err != nil {
			return nil, err
		}

		mediaType, err := image.MediaType()
		if err != nil {
			return nil, err
		}

		if !isImageMediaType(mediaType) {
			return nil, fnerrors.InternalError("%s: unexpected media type: %s", digest.String(), mediaType)
		}

		adds = append(adds, mutate.IndexAddendum{
			Add: image,
			Descriptor: v1.Descriptor{
				MediaType: mediaType,
				Platform: &v1.Platform{
					OS:           d.Platform.OS,
					Architecture: d.Platform.Architecture,
					Variant:      d.Platform.Variant,
				},
			},
		})
	}

	idx := mutate.AppendManifests(mutate.IndexMediaType(empty.Index, types.OCIImageIndex), adds...)

	// The Digest() is requested here to guarantee that the index can indeed be created.
	// This will also mark the digest "computed", which is the closest we can get to a
	// sealed result.
	if _, err := idx.Digest(); err != nil {
		return nil, fnerrors.InternalError("failed to compute image index digest: %w", err)
	}

	return rawImageIndex{idx}, nil
}
