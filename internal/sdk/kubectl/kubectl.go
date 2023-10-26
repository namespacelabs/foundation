// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package kubectl

import (
	"context"
	"fmt"
	"path/filepath"

	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/internal/artifacts"
	"namespacelabs.dev/foundation/internal/artifacts/unpack"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/tasks"
)

const version = "1.26.10"

var Pins = map[string]artifacts.Reference{
	"linux/amd64": {
		URL: fmt.Sprintf("https://dl.k8s.io/release/v%s/bin/linux/amd64/kubectl", version),
		Digest: schema.Digest{
			Algorithm: "sha256",
			Hex:       "93ad44b4072669237247bfbc171be816f08e7e9e4260418d2cfdd0da1704ae86",
		},
	},
	"linux/arm64": {
		URL: fmt.Sprintf("https://dl.k8s.io/release/v%s/bin/linux/arm64/kubectl", version),
		Digest: schema.Digest{
			Algorithm: "sha256",
			Hex:       "5752e3908fa1d338eb1fa99a6f39c6a4c27b065cb459da84e35c4ec718879f14",
		},
	},
	"darwin/arm64": {
		URL: fmt.Sprintf("https://dl.k8s.io/release/v%s/bin/darwin/arm64/kubectl", version),
		Digest: schema.Digest{
			Algorithm: "sha256",
			Hex:       "e356b0262e2c3b2e653ea699149620361cf1381e98732bf173c8243089167605",
		},
	},
	"darwin/amd64": {
		URL: fmt.Sprintf("https://dl.k8s.io/release/v%s/bin/darwin/amd64/kubectl", version),
		Digest: schema.Digest{
			Algorithm: "sha256",
			Hex:       "325a65e3ba0ece81be327f68dfe9132c2c6befd209c0a6a4463cc9668add2e37",
		},
	},
}

type Kubectl string

func EnsureSDK(ctx context.Context, p specs.Platform) (Kubectl, error) {
	sdk, err := SDK(ctx, p)
	if err != nil {
		return "", err
	}

	return compute.GetValue(ctx, sdk)
}

func SDK(ctx context.Context, p specs.Platform) (compute.Computable[Kubectl], error) {
	key := fmt.Sprintf("%s/%s", p.OS, p.Architecture)
	ref, ok := Pins[key]
	if !ok {
		return nil, fnerrors.New("platform not supported: %s", key)
	}

	w := unpack.Unpack("kubectl", unpack.MakeFilesystem("kubectl", 0755, ref))

	return compute.Map(
		tasks.Action("kubectl.ensure").Arg("version", version).HumanReadablef("Ensuring kubectl %s is installed", version),
		compute.Inputs().Computable("kubectl", w),
		compute.Output{},
		func(ctx context.Context, r compute.Resolved) (Kubectl, error) {
			return Kubectl(filepath.Join(compute.MustGetDepValue(r, w, "kubectl").Files, "kubectl")), nil
		}), nil
}
