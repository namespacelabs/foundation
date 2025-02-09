// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package melange

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

const version = "0.19.4"

var Pins = map[string]artifacts.Reference{
	"linux/amd64": {
		URL: fmt.Sprintf("https://github.com/chainguard-dev/melange/releases/download/v%s/melange_%s_linux_amd64.tar.gz", version, version),
		Digest: schema.Digest{
			Algorithm: "sha256",
			Hex:       "940df40fd759b50c9426150496a83a6eff48ef864bd790eeb8b27d3e9bbcb5ff",
		},
	},
	"linux/arm64": {
		URL: fmt.Sprintf("https://github.com/chainguard-dev/melange/releases/download/v%s/melange_%s_linux_arm64.tar.gz", version, version),
		Digest: schema.Digest{
			Algorithm: "sha256",
			Hex:       "f1da4af66164ba9ba9aa83a90fe6080b1800838f2cadc2dc49bc49d6bc266884",
		},
	},
	"darwin/arm64": {
		URL: fmt.Sprintf("https://github.com/chainguard-dev/melange/releases/download/v%s/melange_%s_darwin_arm64.tar.gz", version, version),
		Digest: schema.Digest{
			Algorithm: "sha256",
			Hex:       "9a511cb67618f6782dfb29a44b5ed47a44b184c3ea783e13b418b22f56c696b4",
		},
	},
	"darwin/amd64": {
		URL: fmt.Sprintf("https://github.com/chainguard-dev/melange/releases/download/v%s/melange_%s_darwin_amd64.tar.gz", version, version),
		Digest: schema.Digest{
			Algorithm: "sha256",
			Hex:       "f3306ba66f9f4d83947ecf0b05ae52d2d75a8cfca9e470ad8a4b83430a889c2b",
		},
	},
}

type Melange string

func EnsureSDK(ctx context.Context, p specs.Platform) (Melange, error) {
	sdk, err := SDK(ctx, p)
	if err != nil {
		return "", err
	}

	return compute.GetValue(ctx, sdk)
}

func SDK(ctx context.Context, p specs.Platform) (compute.Computable[Melange], error) {
	key := fmt.Sprintf("%s/%s", p.OS, p.Architecture)
	ref, ok := Pins[key]
	if !ok {
		return nil, fnerrors.New("platform not supported: %s", key)
	}

	w := unpack.Unpack("melange", tarfs.TarGunzip(download.URL(ref)))

	return compute.Map(
		tasks.Action("melange.ensure").Arg("version", version).HumanReadablef("Ensuring melange %s is installed", version),
		compute.Inputs().Computable("melange", w),
		compute.Output{},
		func(ctx context.Context, r compute.Resolved) (Melange, error) {
			return Melange(filepath.Join(compute.MustGetDepValue(r, w, "melange").Files, fmt.Sprintf("melange_%s_%s_%s", version, p.OS, p.Architecture), "melange")), nil
		}), nil
}
