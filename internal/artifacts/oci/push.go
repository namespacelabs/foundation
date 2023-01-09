// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package oci

import (
	"context"
	"reflect"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/std/tasks"
)

func pushImage(ctx context.Context, tag TargetRepository, img v1.Image, trackProgress bool) (v1.Hash, error) {
	digest, err := img.Digest()
	if err != nil {
		return v1.Hash{}, fnerrors.InternalError("digest missing on %q: %w", reflect.TypeOf(img).String(), err)
	}

	ref, err := ParseTag(tag, digest)
	if err != nil {
		return v1.Hash{}, fnerrors.InternalError("failed to parse tag: %w", err)
	}

	remoteOpts, err := RemoteOptsWithAuth(ctx, tag.RegistryAccess, true)
	if err != nil {
		return v1.Hash{}, fnerrors.InternalError("failed to construct remoteops: %w", err)
	}

	if trackProgress {
		var rp RemoteProgress
		tasks.Attachments(ctx).SetProgress(&rp)
		remoteOpts = append(remoteOpts, rp.Track())
	}

	if err := remote.Write(ref, img, remoteOpts...); err != nil {
		return v1.Hash{}, fnerrors.InvocationError("registry", "failed to push image %q: %w", ref, err)
	}

	return digest, nil
}

func pushImageIndex(ctx context.Context, tag TargetRepository, img v1.ImageIndex, trackProgress bool) error {
	digest, err := img.Digest()
	if err != nil {
		return err
	}

	ref, err := ParseTag(tag, digest)
	if err != nil {
		return err
	}

	remoteOpts, err := RemoteOptsWithAuth(ctx, tag.RegistryAccess, true)
	if err != nil {
		return err
	}

	if trackProgress {
		var rp RemoteProgress
		remoteOpts = append(remoteOpts, rp.Track())
		tasks.Attachments(ctx).SetProgress(&rp)
	}

	return remote.WriteIndex(ref, img, remoteOpts...)
}
