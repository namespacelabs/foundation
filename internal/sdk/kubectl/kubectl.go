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

const version = "1.32.3"

var Pins = map[string]artifacts.Reference{
	"linux/amd64": {
		URL: fmt.Sprintf("https://cdn.dl.k8s.io/release/v%s/bin/linux/amd64/kubectl", version),
		Digest: schema.Digest{
			Algorithm: "sha256",
			Hex:       "ab209d0c5134b61486a0486585604a616a5bb2fc07df46d304b3c95817b2d79f",
		},
	},
	"linux/arm64": {
		URL: fmt.Sprintf("https://cdn.dl.k8s.io/release/v%s/bin/linux/arm64/kubectl", version),
		Digest: schema.Digest{
			Algorithm: "sha256",
			Hex:       "6c2c91e760efbf3fa111a5f0b99ba8975fb1c58bb3974eca88b6134bcf3717e2",
		},
	},
	"darwin/arm64": {
		URL: fmt.Sprintf("https://cdn.dl.k8s.io/release/v%s/bin/darwin/arm64/kubectl", version),
		Digest: schema.Digest{
			Algorithm: "sha256",
			Hex:       "a110af64fc31e2360dd0f18e4110430e6eedda1a64f96e9d89059740a7685bbd",
		},
	},
	"darwin/amd64": {
		URL: fmt.Sprintf("https://cdn.dl.k8s.io/release/v%s/bin/darwin/amd64/kubectl", version),
		Digest: schema.Digest{
			Algorithm: "sha256",
			Hex:       "b814c523071cd09e27c88d8c87c0e9b054ca0cf5c2b93baf3127750a4f194d5b",
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
		return nil, fnerrors.Newf("platform not supported: %s", key)
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
