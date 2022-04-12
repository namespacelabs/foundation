// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package oci

import (
	"bytes"
	"context"
	"fmt"
	"reflect"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/devhost"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func ResolveImage(ref string, platform specs.Platform) compute.Computable[Image] {
	return ImageP(ref, &platform)
}

// Returns a Computable which constraints on platform if one is specified.
func ImageP(ref string, platform *specs.Platform) compute.Computable[Image] {
	imageID := ResolveDigest(ref)
	return &fetchImage{imageid: imageID, descriptor: &fetchDescriptor{imageID: imageID}, platform: platform}
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
	imageid    compute.Computable[ImageID]
	descriptor compute.Computable[*RawDescriptor]
	platform   *specs.Platform

	compute.DoScoped[Image] // Need long-lived ctx, as it's captured to fetch Layers.
}

func (r *fetchImage) Action() *tasks.ActionEvent {
	action := tasks.Action("oci.pull-image").Arg("ref", RefFrom(r.imageid))
	if r.platform != nil {
		action = action.Arg("platform", devhost.FormatPlatform(*r.platform))
	}
	return action
}

func (r *fetchImage) Inputs() *compute.In {
	return compute.Inputs().JSON("platform", r.platform).Computable("descriptor", r.descriptor).Computable("imageid", r.imageid)
}

func (r *fetchImage) ImageRef() string {
	return RefFrom(r.imageid)
}

func (r *fetchImage) Compute(ctx context.Context, deps compute.Resolved) (Image, error) {
	descriptor := compute.GetDepValue(deps, r.descriptor, "descriptor")

	switch types.MediaType(descriptor.MediaType) {
	case types.DockerManifestList:
		idx, err := v1.ParseIndexManifest(bytes.NewReader(descriptor.RawManifest))
		if err != nil {
			return nil, fnerrors.BadInputError("expected to parse an image index: %w", err)
		}

		return imageForPlatform(idx, r.platform, func(h v1.Hash) (Image, error) {
			d := ImageID{Repository: descriptor.Repository, Digest: h.String()}
			// When we do a recursive lookup we don't constrain platform anymore, as more
			// often than not images that are referred to from an index don't carry a platform
			// specification.
			return compute.GetValue(ctx, ImageP(d.ImageRef(), nil))
		})

	case types.DockerManifestSchema2:
		imageid := compute.GetDepValue(deps, r.imageid, "imageid")
		name, err := name.NewDigest(imageid.RepoAndDigest())
		if err != nil {
			return nil, fnerrors.InternalError("failed to parse: %w", err)
		}

		img, err := remote.Image(name, RemoteOpts(ctx)...)
		if err != nil {
			return nil, fnerrors.InvocationError("failed to fetch image: %w", err)
		}

		return img, nil
	}

	return nil, fnerrors.BadInputError("unexpected media type: %s (expected image or image index)", descriptor.MediaType)
}

func fetchRemoteDescriptor(ctx context.Context, ref string, moreOpts ...remote.Option) (*remote.Descriptor, error) {
	opts := RemoteOpts(ctx)
	opts = append(opts, moreOpts...)

	baseRef, err := name.ParseReference(ref)
	if err != nil {
		return nil, err
	}

	return remote.Get(baseRef, opts...)
}

type HasImageRef interface {
	ImageRef() string
}

func RefFrom(c any) string {
	if x, ok := compute.Unwrap(c); ok {
		return RefFrom(x)
	}

	switch x := c.(type) {
	case HasImageRef:
		return x.ImageRef()
	}
	return fmt.Sprintf("<was expecting Image() got %q>", reflect.TypeOf(c).String())
}

type fetchDescriptor struct {
	imageID compute.Computable[ImageID]
	compute.LocalScoped[*RawDescriptor]
}

func (r *fetchDescriptor) Inputs() *compute.In {
	return compute.Inputs().Computable("resolved", r.imageID)
}

func (r *fetchDescriptor) Action() *tasks.ActionEvent {
	return tasks.Action("oci.fetch-descriptor").Arg("ref", RefFrom(r.imageID))
}

func (r *fetchDescriptor) ImageRef() string {
	return RefFrom(r.imageID)
}

func (r *fetchDescriptor) Compute(ctx context.Context, deps compute.Resolved) (*RawDescriptor, error) {
	digest := compute.GetDepValue(deps, r.imageID, "resolved")
	d, err := fetchRemoteDescriptor(ctx, digest.ImageRef())
	if err != nil {
		return nil, fnerrors.InvocationError("failed to fetch descriptor: %w", err)
	}

	res := &RawDescriptor{
		Repository:  digest.Repository,
		MediaType:   string(d.MediaType),
		RawManifest: d.Manifest,
	}

	// Also cache the config manifest, if this is an image.
	if d.MediaType == types.DockerManifestSchema2 {
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
