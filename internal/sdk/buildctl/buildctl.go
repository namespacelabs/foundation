// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package buildctl

import (
	"context"
	"fmt"
	"path/filepath"

	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/internal/artifacts"
	"namespacelabs.dev/foundation/internal/artifacts/download"
	"namespacelabs.dev/foundation/internal/artifacts/unpack"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs/tarfs"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/tasks"
)

const version = "v0.11.2"

var Pins = map[string]artifacts.Reference{
	"linux/amd64": {
		URL: fmt.Sprintf("https://github.com/moby/buildkit/releases/download/%s/buildkit-%s.linux-amd64.tar.gz", version, version),
		Digest: schema.Digest{
			Algorithm: "sha256",
			Hex:       "6d0fe3f1ec2dce4ed2a5c9baf05fb279225b3b0e3bbee4092304fe284ca7fc47",
		},
	},
	"linux/arm64": {
		URL: fmt.Sprintf("https://github.com/moby/buildkit/releases/download/%s/buildkit-%s.linux-arm64.tar.gz", version, version),
		Digest: schema.Digest{
			Algorithm: "sha256",
			Hex:       "8a8c2274852ea4bac6ccf1862a46679e93c013de8c5c0434a3040bab2e0a42a7",
		},
	},
	"darwin/arm64": {
		URL: fmt.Sprintf("https://github.com/moby/buildkit/releases/download/%s/buildkit-%s.darwin-arm64.tar.gz", version, version),
		Digest: schema.Digest{
			Algorithm: "sha256",
			Hex:       "7606f2aee4898170edfeee0ea0044cd99905f480a84b8c621f878bb9493d4f09",
		},
	},
	"darwin/amd64": {
		URL: fmt.Sprintf("https://github.com/moby/buildkit/releases/download/%s/buildkit-%s.darwin-amd64.tar.gz", version, version),
		Digest: schema.Digest{
			Algorithm: "sha256",
			Hex:       "bf0f536f8ec775392ed7542acaf5390496e6d931df6758a17f341d595e0c7691",
		},
	},
}

type Buildctl string

func EnsureSDK(ctx context.Context, p specs.Platform) (Buildctl, error) {
	sdk, err := SDK(ctx, p)
	if err != nil {
		return "", err
	}

	return compute.GetValue(ctx, sdk)
}

func SDK(ctx context.Context, p specs.Platform) (compute.Computable[Buildctl], error) {
	key := fmt.Sprintf("%s/%s", p.OS, p.Architecture)
	ref, ok := Pins[key]
	if !ok {
		return nil, fnerrors.Newf("platform not supported: %s", key)
	}

	w := unpack.Unpack("buildctl", tarfs.TarGunzip(download.URL(ref)))

	return compute.Map(
		tasks.Action("buildctl.ensure").Arg("version", version).HumanReadablef("Ensuring buildctl %s is installed", version),
		compute.Inputs().Computable("buildctl", w),
		compute.Output{},
		func(ctx context.Context, r compute.Resolved) (Buildctl, error) {
			return Buildctl(filepath.Join(compute.MustGetDepValue(r, w, "buildctl").Files, "bin/buildctl")), nil
		}), nil
}
