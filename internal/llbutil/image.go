// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package llbutil

import (
	"context"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/moby/buildkit/client/llb"
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

	return OCILayout(d), nil
}

func OCILayout(digest v1.Hash, opts ...llb.OCILayoutOption) llb.State {
	opts = append(opts, llb.OCIStore("", "cache"))
	// Another buildkit-ism. OCILayout resolution is digest based, but because
	// it uses reference.Parse for parsing, it needs an arbitrary
	// not-single-word key to parse as a host.
	return llb.OCILayout("cache/cache@"+digest.String(), opts...)
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
