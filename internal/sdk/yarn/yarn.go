// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package yarn

import (
	"context"
	"fmt"
	"path/filepath"

	"namespacelabs.dev/foundation/internal/artifacts"
	"namespacelabs.dev/foundation/internal/artifacts/unpack"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/tasks"
)

const version = "3.2.1"

var Pin = artifacts.Reference{
	URL: fmt.Sprintf("https://repo.yarnpkg.com/%s/packages/yarnpkg-cli/bin/yarn.js", version),
	Digest: schema.Digest{
		Algorithm: "sha256",
		Hex:       "f1c9f039ab3b236c7abb7bafa0a0f266aa6bc5a3dbe7c684b561f03c0005d043",
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
	w := unpack.Unpack("yarn", unpack.MakeFilesystem("yarn.js", 0755, Pin))

	return compute.Map(
		tasks.Action("yarn.ensure").Arg("version", version).HumanReadablef("Ensuring yarn %s is installed", version),
		compute.Inputs().Computable("yarn", w),
		compute.Output{},
		func(ctx context.Context, r compute.Resolved) (Yarn, error) {
			return Yarn(filepath.Join(compute.MustGetDepValue(r, w, "yarn").Files, "yarn.js")), nil
		}), nil
}
