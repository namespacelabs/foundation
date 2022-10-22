// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package octant

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"namespacelabs.dev/foundation/internal/artifacts"
	"namespacelabs.dev/foundation/internal/artifacts/download"
	"namespacelabs.dev/foundation/internal/artifacts/unpack"
	"namespacelabs.dev/foundation/internal/bytestream"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs/tarfs"
	"namespacelabs.dev/foundation/internal/parsing/devhost"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/tasks"
)

const version = "0.25.1"

var Pins = map[string]artifacts.Reference{
	"linux/amd64": {
		URL: fmt.Sprintf("https://github.com/vmware-tanzu/octant/releases/download/v%s/octant_%s_Linux-64bit.tar.gz", version, version),
		Digest: schema.Digest{
			Algorithm: "sha256",
			Hex:       "b12bb6752e43f4e0fe54278df8e98dee3439c4066f66cdb7a0ca4a1c7d8eaa1e",
		},
	},
	"linux/arm64": {
		URL: fmt.Sprintf("https://github.com/vmware-tanzu/octant/releases/download/v%s/octant_%s_Linux-arm64.tar.gz", version, version),
		Digest: schema.Digest{
			Algorithm: "sha256",
			Hex:       "a3eb4973a0c869267e3916bd43e0b41b2bbc73b898376b795a617299c7b2a623",
		},
	},
	"darwin/arm64": {
		URL: fmt.Sprintf("https://github.com/vmware-tanzu/octant/releases/download/v%s/octant_%s_macOS-arm64.tar.gz", version, version),
		Digest: schema.Digest{
			Algorithm: "sha256",
			Hex:       "9528d1a3e00f1bf0180457a347aac6963dfdc3faa3a85970b93932a352fb38cf",
		},
	},
	"darwin/amd64": {
		URL: fmt.Sprintf("https://github.com/vmware-tanzu/octant/releases/download/v%s/octant_%s_macOS-64bit.tar.gz", version, version),
		Digest: schema.Digest{
			Algorithm: "sha256",
			Hex:       "97b1510362d99c24eeef98b61ca327e6e5323c99a1c774bc8e60751d3c923b33",
		},
	},
}

type Octant string

func EnsureSDK(ctx context.Context) (Octant, error) {
	sdk, err := SDK(ctx)
	if err != nil {
		return "", err
	}

	return compute.GetValue(ctx, sdk)
}

func SDK(ctx context.Context) (compute.Computable[Octant], error) {
	platform := devhost.RuntimePlatform()
	key := fmt.Sprintf("%s/%s", platform.OS, platform.Architecture)
	ref, ok := Pins[key]
	if !ok {
		return nil, fnerrors.UserError(nil, "platform not supported: %s", key)
	}

	w := unpack.Unpack("octant", tarfs.TarGunzip(download.URL(ref)))

	return compute.Map(
		tasks.Action("octant.ensure").Arg("version", version).HumanReadablef("Ensuring octant %s is installed", version),
		compute.Inputs().Computable("octant", w),
		compute.Output{},
		func(ctx context.Context, r compute.Resolved) (Octant, error) {
			dir := strings.TrimSuffix(filepath.Base(ref.URL), ".tar.gz")

			return Octant(filepath.Join(compute.MustGetDepValue(r, w, "octant").Files, dir, "octant")), nil
		}), nil
}

func AllDownloads() []compute.Computable[bytestream.ByteStream] {
	var downloads []compute.Computable[bytestream.ByteStream]
	for _, v := range Pins {
		downloads = append(downloads, download.URL(v))
	}
	return downloads
}
