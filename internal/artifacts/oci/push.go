// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package oci

import (
	"context"
	"errors"
	"net/http"
	"reflect"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func pushImage(ctx context.Context, tag AllocatedName, img v1.Image) (ImageID, error) {
	digest, err := img.Digest()
	if err != nil {
		return ImageID{}, fnerrors.InternalError("digest missing on %q: %w", reflect.TypeOf(img).String(), err)
	}

	ref, err := ParseTag(tag)
	if err != nil {
		return ImageID{}, fnerrors.InternalError("failed to parse tag: %w", err)
	}

	var rp RemoteProgress
	if err := tasks.Action("oci.push-image").Progress(&rp).Arg("ref", ref).Run(ctx, func(ctx context.Context) error {
		remoteOpts := RemoteOptsForWriting(ctx, tag.Keychain)
		remoteOpts = append(remoteOpts, rp.Track())

		if err := remote.Write(ref, img, remoteOpts...); err != nil {
			if ok, nerr := checkUnauthorized(err, ref); ok {
				return nerr
			}

			return fnerrors.InvocationError("failed to push to registry %q: %w", ref, err)
		}

		return nil
	}); err != nil {
		return ImageID{}, err
	}

	return tag.WithDigest(digest), nil
}

func checkUnauthorized(err error, ref name.Tag) (bool, error) {
	terr := &transport.Error{}
	if errors.As(err, &terr) {
		if terr.StatusCode == http.StatusUnauthorized || terr.StatusCode == http.StatusForbidden {
			return true, fnerrors.UsageError("Try running `fn refresh-creds`.",
				"Failed to upload image to %q, perhaps you are missing up-to-date credentials?", ref.Registry)
		}
	}

	return false, err
}

func pushImageIndex(ctx context.Context, tag AllocatedName, img v1.ImageIndex) error {
	ref, err := ParseTag(tag)
	if err != nil {
		return err
	}

	var rp RemoteProgress
	return tasks.Action("oci.write-image-index").Progress(&rp).Arg("ref", ref).Run(ctx, func(ctx context.Context) error {
		remoteOpts := RemoteOptsForWriting(ctx, tag.Keychain)
		remoteOpts = append(remoteOpts, rp.Track())

		if err := remote.WriteIndex(ref, img, remoteOpts...); err != nil {
			if ok, nerr := checkUnauthorized(err, ref); ok {
				return nerr
			}
			return err
		}
		return nil
	})
}
