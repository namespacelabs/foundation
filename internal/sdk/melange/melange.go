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

const version = "0.28.0"

var Pins = map[string]artifacts.Reference{
	"linux/amd64": {
		URL: fmt.Sprintf("https://github.com/chainguard-dev/melange/releases/download/v%s/melange_%s_linux_amd64.tar.gz", version, version),
		Digest: schema.Digest{
			Algorithm: "sha256",
			Hex:       "6941deb77c43813007a126250ad43f1d52296ae27a523cc7d064d2772bd21653",
		},
	},
	"linux/arm64": {
		URL: fmt.Sprintf("https://github.com/chainguard-dev/melange/releases/download/v%s/melange_%s_linux_arm64.tar.gz", version, version),
		Digest: schema.Digest{
			Algorithm: "sha256",
			Hex:       "a848a837d49668fc1cca404e158b7d6d05648827b802eebc34de14f6ade7dfb0",
		},
	},
	"darwin/arm64": {
		URL: fmt.Sprintf("https://github.com/chainguard-dev/melange/releases/download/v%s/melange_%s_darwin_arm64.tar.gz", version, version),
		Digest: schema.Digest{
			Algorithm: "sha256",
			Hex:       "0e13f30e08fac51c0b3048f210bbcb1993f5cb4343822f7063c7279c154d802f",
		},
	},
	"darwin/amd64": {
		URL: fmt.Sprintf("https://github.com/chainguard-dev/melange/releases/download/v%s/melange_%s_darwin_amd64.tar.gz", version, version),
		Digest: schema.Digest{
			Algorithm: "sha256",
			Hex:       "e488edc9e258d7f4628f0cee5bc115c539dccb8741cc8a3b961613459b6dad8c",
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
		return nil, fnerrors.Newf("platform not supported: %s", key)
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
