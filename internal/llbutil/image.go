// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package llbutil

import (
	"context"

	"github.com/moby/buildkit/client/llb"
	"github.com/opencontainers/go-digest"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/compute"
)

var useOCILayout = false

func Prebuilt(ctx context.Context, ref string, platform specs.Platform) (llb.State, error) {
	if !useOCILayout {
		return Image(ref, platform), nil
	}

	image, err := compute.GetValue(ctx, oci.ImageP(ref, &platform, oci.RegistryAccess{PublicImage: true}))
	if err != nil {
		return llb.State{}, err
	}

	cachedImage, err := oci.EnsureCached(ctx, image)
	if err != nil {
		return llb.State{}, err
	}

	d, err := cachedImage.Digest()
	if err != nil {
		return llb.State{}, err
	}

	return llb.OCILayout("cache", digest.Digest(d.String())), nil
}

func Image(image string, platform specs.Platform) llb.State {
	return llb.Image(image, llb.ResolveModeDefault, llb.Platform(platform), ImageName(image, &platform))
}

func ImageName(image string, platform *specs.Platform) llb.ConstraintsOpt {
	if platform == nil {
		return llb.WithCustomNamef("Image: %s (host)", image)
	}
	return llb.WithCustomNamef("Image: %s (%s/%s)", image, platform.OS, platform.Architecture)
}
