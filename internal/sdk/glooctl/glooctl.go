// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package glooctl

import (
	"context"
	"fmt"
	"path/filepath"

	"namespacelabs.dev/foundation/internal/artifacts"
	"namespacelabs.dev/foundation/internal/artifacts/download"
	"namespacelabs.dev/foundation/internal/artifacts/unpack"
	"namespacelabs.dev/foundation/internal/bytestream"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/devhost"
	"namespacelabs.dev/foundation/workspace/tasks"
)

const version = "1.11.2"

var Pins = map[string]artifacts.Reference{
	"linux/amd64": {
		URL: fmt.Sprintf("https://github.com/solo-io/gloo/releases/download/v%s/glooctl-linux-amd64", version),
		Digest: schema.Digest{
			Algorithm: "sha256",
			Hex:       "b952ba1fb37743d81429d76ae895d2a5a33a5435c1a7d4a850d82972b61dc7ae",
		},
	},
	"darwin/amd64": {
		URL: fmt.Sprintf("https://github.com/solo-io/gloo/releases/download/v%s/glooctl-darwin-amd64", version),
		Digest: schema.Digest{
			Algorithm: "sha256",
			Hex:       "5051f6a1b9162bac3382626b2c58f7fcc6213d9f084c6883929924bce3733da1",
		},
	},
}

type Glooctl string

func EnsureSDK(ctx context.Context) (Glooctl, error) {
	sdk, err := SDK(ctx)
	if err != nil {
		return "", err
	}

	return compute.GetValue(ctx, sdk)
}

func SDK(ctx context.Context) (compute.Computable[Glooctl], error) {
	platform := devhost.RuntimePlatform()
	key := fmt.Sprintf("%s/%s", platform.OS, platform.Architecture)
	ref, ok := Pins[key]
	if !ok {
		return nil, fnerrors.UserError(nil, "platform not supported: %s", key)
	}

	w := unpack.Unpack("glooctl", unpack.MakeFilesystem("glooctl", 0755, ref))

	return compute.Map(
		tasks.Action("glooctl.ensure").Arg("version", version).HumanReadablef("Ensuring glooctl %s is installed", version),
		compute.Inputs().Computable("glooctl", w),
		compute.Output{},
		func(ctx context.Context, r compute.Resolved) (Glooctl, error) {
			return Glooctl(filepath.Join(compute.MustGetDepValue(r, w, "glooctl").Files, "glooctl")), nil
		}), nil
}

func AllDownloads() []compute.Computable[bytestream.ByteStream] {
	var downloads []compute.Computable[bytestream.ByteStream]
	for _, v := range Pins {
		downloads = append(downloads, download.URL(v))
	}
	return downloads
}
