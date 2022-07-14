// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package oci

import (
	"context"

	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"namespacelabs.dev/foundation/workspace/tasks"
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

		return mutate.ConfigFile(img, cfg)
	})
}
