// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package oci

import (
	"context"
	"reflect"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func pushImage(ctx context.Context, tag AllocatedName, img v1.Image) (ImageID, error) {
	digest, err := img.Digest()
	if err != nil {
		return ImageID{}, fnerrors.InternalError("digest missing on %q: %w", reflect.TypeOf(img).String(), err)
	}

	ref, err := ParseTag(tag, digest)
	if err != nil {
		return ImageID{}, fnerrors.InternalError("failed to parse tag: %w", err)
	}

	var rp RemoteProgress
	if err := tasks.Action("oci.push-image").Progress(&rp).Arg("ref", ref).Run(ctx, func(ctx context.Context) error {
		remoteOpts := WriteRemoteOptsWithAuth(ctx, tag.Keychain)
		remoteOpts = append(remoteOpts, rp.Track())

		if err := remote.Write(ref, img, remoteOpts...); err != nil {
			return fnerrors.InvocationError("failed to push to registry %q: %w", ref, err)
		}

		return nil
	}); err != nil {
		return ImageID{}, err
	}

	return tag.WithDigest(digest), nil
}

func pushImageIndex(ctx context.Context, tag AllocatedName, img v1.ImageIndex) error {
	digest, err := img.Digest()
	if err != nil {
		return err
	}

	ref, err := ParseTag(tag, digest)
	if err != nil {
		return err
	}

	var rp RemoteProgress
	return tasks.Action("oci.write-image-index").Progress(&rp).Arg("ref", ref).Run(ctx, func(ctx context.Context) error {
		remoteOpts := WriteRemoteOptsWithAuth(ctx, tag.Keychain)
		remoteOpts = append(remoteOpts, rp.Track())

		if err := remote.WriteIndex(ref, img, remoteOpts...); err != nil {
			return err
		}
		return nil
	})
}
