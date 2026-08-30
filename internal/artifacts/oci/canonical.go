// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package oci

import (
	"context"

	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"namespacelabs.dev/foundation/std/tasks"
)

func WithCanonicalManifest(ctx context.Context, img Image) (Image, error) {
	digest, err := img.Digest()
	if err != nil {
		return nil, err
	}

	return tasks.Return(ctx, tasks.Action("oci.image.make-canonical").Arg("digest", digest), func(ctx context.Context) (Image, error) {
		// mutate.Canonical() resets the build timestamps for each layer. That's
		// too expensive, as it requires decompressing all layers. So we let the original
		// layers as-is, but clear other sources of non-determinism out.

		cf, err := img.ConfigFile()
		if err != nil {
			return nil, err
		}

		// Get rid of host-dependent random config
		cfg := cf.DeepCopy()

		cfg.Container = ""
		cfg.Config.Hostname = ""
		cfg.DockerVersion = ""

		resulting, err := mutate.ConfigFile(img, cfg)
		if err != nil {
			return nil, err
		}

		resultingDigest, err := resulting.Digest()
		if err == nil {
			tasks.Attachments(ctx).AddResult("transformed", resultingDigest)
		}

		return resulting, err
	})
}

func Canonical(ctx context.Context, original Image) (Image, error) {
	img, err := WithCanonicalManifest(ctx, original)
	if err != nil {
		return nil, err
	}

	AttachDigestToAction(ctx, img)
	return img, nil
}

func AttachDigestToAction(ctx context.Context, img Image) {
	digest, err := img.Digest()
	if err != nil {
		tasks.Attachments(ctx).AddResult("digest", "<failed>")
	} else {
		tasks.Attachments(ctx).AddResult("digest", digest)
	}

	cfgName, err := img.ConfigName()
	if err != nil {
		tasks.Attachments(ctx).AddResult("config", "<failed>")
	} else {
		tasks.Attachments(ctx).AddResult("config", cfgName)
	}
}
