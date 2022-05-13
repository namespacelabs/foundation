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
	"namespacelabs.dev/foundation/workspace/dirs"
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
	cacheDir, err := dirs.SDKCache("yarn")
	if err != nil {
		return nil, err
	}

	yarnPath := filepath.Join(cacheDir, "yarn")
	written := unpack.WriteLocal(yarnPath, 0644, Pin)

	return compute.Map(
		tasks.Action("yarn.ensure").Arg("version", version).HumanReadablef("Ensuring yarn %s is installed", version),
		compute.Inputs().Computable("yarn", written),
		compute.Output{},
		func(ctx context.Context, r compute.Resolved) (Yarn, error) {
			return Yarn(compute.GetDepValue(r, written, "yarn")), nil
		}), nil
}

func AllDownloads() []compute.Computable[bytestream.ByteStream] {
	return []compute.Computable[bytestream.ByteStream]{download.URL(Pin)}
}
