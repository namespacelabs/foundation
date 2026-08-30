// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package baseimage

import (
	"context"

	"github.com/moby/buildkit/client/llb"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/build/binary"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/llbutil"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

func Load(ctx context.Context, env pkggraph.SealedContext, ref *schema.PackageRef, plat specs.Platform) (oci.NamedImage, error) {
	p, err := binary.Load(ctx, env, env, ref, binary.BuildImageOpts{UsePrebuilts: true, Platforms: []specs.Platform{plat}})
	if err != nil {
		return nil, err
	}

	base, err := p.Image(ctx, env)
	if err != nil {
		return nil, err
	}

	return oci.MakeNamedImage(ref.Canonical(), compute.Transform("resolve-platform", base, func(_ context.Context, img oci.ResolvableImage) (oci.Image, error) {
		return img.ImageForPlatform(plat)
	})), nil
}

func State(ctx context.Context, base oci.NamedImage) (llb.State, error) {
	x, err := compute.GetValue(ctx, base.Image())
	if err != nil {
		return llb.State{}, err
	}

	return llbutil.OCILayoutFromImage(ctx, x)
}
