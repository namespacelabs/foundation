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

const version = "1.26.9"

var Pins = map[string]artifacts.Reference{
	"linux/amd64": {
		URL: fmt.Sprintf("https://cdn.dl.k8s.io/release/v%s/bin/linux/amd64/kubectl", version),
		Digest: schema.Digest{
			Algorithm: "sha256",
			Hex:       "98ea4a13895e54ba24f57e0d369ff6be0d3906895305d5390197069b1da12ae2",
		},
	},
	"linux/arm64": {
		URL: fmt.Sprintf("https://cdn.dl.k8s.io/release/v%s/bin/linux/arm64/kubectl", version),
		Digest: schema.Digest{
			Algorithm: "sha256",
			Hex:       "f945c63220b393ddf8df67d87e67ff74b7f56219a670dee38bc597a078588e90",
		},
	},
	"darwin/arm64": {
		URL: fmt.Sprintf("https://cdn.dl.k8s.io/release/v%s/bin/darwin/arm64/kubectl", version),
		Digest: schema.Digest{
			Algorithm: "sha256",
			Hex:       "717e6e4cc9815c3dc4fd115d85d63a47f50bca3cd96815d1b22da9a6ff8fa90c",
		},
	},
	"darwin/amd64": {
		URL: fmt.Sprintf("https://cdn.dl.k8s.io/release/v%s/bin/darwin/amd64/kubectl", version),
		Digest: schema.Digest{
			Algorithm: "sha256",
			Hex:       "0644a191aac832bfef27e74f315af4ec8c2487df44ee537a1243bc58b583fd73",
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
