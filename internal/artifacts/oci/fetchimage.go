// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package oci

import (
	"bytes"
	"context"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/parsing/devhost"
	"namespacelabs.dev/foundation/std/tasks"
)

func ResolveImage(ref string, platform specs.Platform) NamedImage {
	return MakeNamedImage(ref, ImageP(ref, &platform, ResolveOpts{}))
}

// Returns a Computable which constraints on platform if one is specified.
func ImageP(ref string, platform *specs.Platform, opts ResolveOpts) compute.Computable[Image] {
	imageID := ResolveDigest(ref, opts)
	return &fetchImage{
		imageid:    imageID,
		descriptor: &fetchDescriptor{imageID: imageID, opts: opts},
		platform:   platform,
	}
}

func toV1Plat(p *specs.Platform) *v1.Platform {
	if p == nil {
		return nil
	}

	return &v1.Platform{
		OS:           p.OS,
		Architecture: p.Architecture,
		// XXX handle variant.
	}
}

type fetchImage struct {
	imageid    NamedImageID
	descriptor compute.Computable[*RawDescriptor]
	platform   *specs.Platform
	opts       ResolveOpts // Does not affect output.

	compute.DoScoped[Image] // Need long-lived ctx, as it's captured to fetch Layers.
}

func (r *fetchImage) Action() *tasks.ActionEvent {
	action := tasks.Action("oci.pull-image").Arg("ref", r.imageid.Description())
	if r.platform != nil {
		action = action.Arg("platform", devhost.FormatPlatform(*r.platform))
	}
	return action
}

func (r *fetchImage) Inputs() *compute.In {
	return compute.Inputs().JSON("platform", r.platform).Computable("descriptor", r.descriptor).Computable("imageid", r.imageid.ImageID())
}

func (r *fetchImage) Compute(ctx context.Context, deps compute.Resolved) (Image, error) {
	descriptor := compute.MustGetDepValue(deps, r.descriptor, "descriptor")

	switch {
	case isIndexMediaType(types.MediaType(descriptor.MediaType)):
		idx, err := v1.ParseIndexManifest(bytes.NewReader(descriptor.RawManifest))
		if err != nil {
			return nil, fnerrors.BadInputError("expected to parse an image index: %w", err)
		}

		tasks.Attachments(ctx).AddResult("index", true)

		return imageForPlatform(idx, r.platform, func(h v1.Hash) (Image, error) {
			img, err := loadFromCache(ctx, compute.Cache(ctx), h)
			if err != nil {
				return nil, err
			}

			if img != nil {
				return img, nil
			}

			d := ImageID{Repository: descriptor.Repository, Digest: h.String()}
			fetched, err := fetchRemoteImage(ctx, d, r.opts)
			if err != nil {
				return nil, err
			}

			// Best effort caching.
			_ = writeImage(ctx, compute.Cache(ctx), fetched)
			return fetched, nil
		})

	case isImageMediaType(types.MediaType(descriptor.MediaType)):
		imageid := compute.MustGetDepValue(deps, r.imageid.ImageID(), "imageid")

		return fetchRemoteImage(ctx, imageid, r.opts)
	}

	return nil, fnerrors.BadInputError("unexpected media type: %s (expected image or image index)", descriptor.MediaType)
}

func fetchRemoteImage(ctx context.Context, imageid ImageID, opts ResolveOpts) (Image, error) {
	ref, remoteOpts, err := ParseRefAndKeychain(ctx, imageid.RepoAndDigest(), opts)
	if err != nil {
		return nil, fnerrors.InternalError("%s: failed to parse: %w", imageid.RepoAndDigest(), err)
	}

	img, err := remote.Image(ref, remoteOpts...)
	if err != nil {
		return nil, fnerrors.InvocationError("failed to fetch image: %w", err)
	}

	return img, nil
}

func fetchRemoteDescriptor(ctx context.Context, imageRef string, opts ResolveOpts) (*remote.Descriptor, error) {
	ref, remoteOpts, err := ParseRefAndKeychain(ctx, imageRef, opts)
	if err != nil {
		return nil, err
	}

	return remote.Get(ref, remoteOpts...)
}

func ParseRef(imageRef string, insecure bool) (name.Reference, error) {
	var nameOpts []name.Option
	if insecure {
		nameOpts = append(nameOpts, name.Insecure)
	}

	return name.ParseReference(imageRef, nameOpts...)
}

type fetchDescriptor struct {
	imageID NamedImageID
	opts    ResolveOpts
	compute.LocalScoped[*RawDescriptor]
}

func (r *fetchDescriptor) Inputs() *compute.In {
	return compute.Inputs().Computable("resolved", r.imageID.ImageID()).JSON("opts", r.opts)
}

func (r *fetchDescriptor) Action() *tasks.ActionEvent {
	return tasks.Action("oci.fetch-descriptor").Arg("ref", r.imageID.Description())
}

func (r *fetchDescriptor) Compute(ctx context.Context, deps compute.Resolved) (*RawDescriptor, error) {
	digest := compute.MustGetDepValue(deps, r.imageID.ImageID(), "resolved")
	d, err := fetchRemoteDescriptor(ctx, digest.ImageRef(), r.opts)
	if err != nil {
		return nil, fnerrors.InvocationError("failed to fetch descriptor: %w", err)
	}

	res := &RawDescriptor{
		Repository:  digest.Repository,
		MediaType:   string(d.MediaType),
		RawManifest: d.Manifest,
	}

	// Also cache the config manifest, if this is an image.
	if isImageMediaType(d.MediaType) {
		img, err := d.Image()
		if err != nil {
			return nil, fnerrors.BadInputError("expected an image: %w", err)
		}
		res.RawConfig, err = img.RawConfigFile()
		if err != nil {
			return nil, fnerrors.BadInputError("failed to fetch raw image config: %w", err)
		}
	}

	return res, nil
}
