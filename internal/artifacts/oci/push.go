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
	"namespacelabs.dev/foundation/std/tasks"
)

func pushImage(ctx context.Context, tag AllocatedName, img v1.Image, trackProgress bool) (ImageID, error) {
	digest, err := img.Digest()
	if err != nil {
		return ImageID{}, fnerrors.InternalError("digest missing on %q: %w", reflect.TypeOf(img).String(), err)
	}

	ref, err := ParseTag(tag, digest)
	if err != nil {
		return ImageID{}, fnerrors.InternalError("failed to parse tag: %w", err)
	}

	remoteOpts := WriteRemoteOptsWithAuth(ctx, tag.Keychain)
	if trackProgress {
		var rp RemoteProgress
		tasks.Attachments(ctx).SetProgress(&rp)
		remoteOpts = append(remoteOpts, rp.Track())
	}

	if err := remote.Write(ref, img, remoteOpts...); err != nil {
		return ImageID{}, fnerrors.InvocationError("failed to push to registry %q: %w", ref, err)
	}

	return tag.WithDigest(digest), nil
}

func pushImageIndex(ctx context.Context, tag AllocatedName, img v1.ImageIndex, trackProgress bool) error {
	digest, err := img.Digest()
	if err != nil {
		return err
	}

	ref, err := ParseTag(tag, digest)
	if err != nil {
		return err
	}

	remoteOpts := WriteRemoteOptsWithAuth(ctx, tag.Keychain)
	if trackProgress {
		var rp RemoteProgress
		remoteOpts = append(remoteOpts, rp.Track())
		tasks.Attachments(ctx).SetProgress(&rp)
	}

	return remote.WriteIndex(ref, img, remoteOpts...)
}
