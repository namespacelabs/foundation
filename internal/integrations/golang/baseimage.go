// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package golang

import (
	"context"

	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/build/binary"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

var baseImageRef = schema.MakePackageRef("namespacelabs.dev/foundation/library/golang/baseimage", "baseimage")

func baseProdImage(ctx context.Context, env pkggraph.SealedContext, plat specs.Platform) (oci.NamedImage, error) {
	p, err := binary.Load(ctx, env, env, baseImageRef, binary.BuildImageOpts{UsePrebuilts: true, Platforms: []specs.Platform{plat}})
	if err != nil {
		return nil, err
	}

	base, err := p.Image(ctx, env)
	if err != nil {
		return nil, err
	}

	return oci.MakeNamedImage(baseImageRef.Canonical(), compute.Transform("resolve-platform", base, func(_ context.Context, img oci.ResolvableImage) (oci.Image, error) {
		return img.ImageForPlatform(plat)
	})), nil
}
