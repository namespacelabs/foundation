// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package yarn

import (
	"context"
	"path/filepath"

	"namespacelabs.dev/foundation/internal/artifacts"
	"namespacelabs.dev/foundation/internal/artifacts/download"
	"namespacelabs.dev/foundation/internal/artifacts/unpack"
	"namespacelabs.dev/foundation/internal/bytestream"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/tasks"
)

const version = "3.2.0"

var Pin = artifacts.Reference{
	URL: "https://repo.yarnpkg.com/3.2.0/packages/yarnpkg-cli/bin/yarn.js",
	Digest: schema.Digest{
		Algorithm: "sha256",
		Hex:       "99a7f42f678b8ccd9c8e97a9e65fe7a5043033dd0e074e6d1f13fbe2d5ff2734",
	},
}

type Yarn string

func EnsureSDK(ctx context.Context) (Yarn, error) {
	sdk, err := SDK(ctx)
	if err != nil {
		return "", err
	}

	return compute.GetValue(ctx, sdk)
}

func SDK(ctx context.Context) (compute.Computable[Yarn], error) {
	w := unpack.Unpack(unpack.MakeFilesystem("yarn.js", 0755, Pin))

	return compute.Map(
		tasks.Action("yarn.ensure").Arg("version", version).HumanReadablef("Ensuring yarn %s is installed", version),
		compute.Inputs().Computable("yarn", w),
		compute.Output{},
		func(ctx context.Context, r compute.Resolved) (Yarn, error) {
			return Yarn(filepath.Join(compute.GetDepValue(r, w, "yarn").Files, "yarn.js")), nil
		}), nil
}

func AllDownloads() []compute.Computable[bytestream.ByteStream] {
	return []compute.Computable[bytestream.ByteStream]{download.URL(Pin)}
}
