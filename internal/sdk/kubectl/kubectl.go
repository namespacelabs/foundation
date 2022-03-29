// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubectl

import (
	"context"
	"fmt"
	"path/filepath"

	"namespacelabs.dev/foundation/internal/artifacts"
	"namespacelabs.dev/foundation/internal/artifacts/download"
	"namespacelabs.dev/foundation/internal/artifacts/unpack"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/devhost"
	"namespacelabs.dev/foundation/workspace/dirs"
	"namespacelabs.dev/foundation/workspace/tasks"
)

const version = "1.13.5"

var Pins = map[string]artifacts.Reference{
	"linux/amd64": {
		URL: fmt.Sprintf("https://dl.k8s.io/release/v%s/bin/linux/amd64/kubectl", version),
		Digest: schema.Digest{
			Algorithm: "sha256",
			Hex:       "3b0ddcde72fd6ec30675f2d0500b3aff43a0bfd580602bb1c5c75c4072242f35",
		},
	},
	"linux/arm64": {
		URL: fmt.Sprintf("https://dl.k8s.io/release/v%s/bin/linux/arm64/kubectl", version),
		Digest: schema.Digest{
			Algorithm: "sha256",
			Hex:       "46298ad006f76aa3a07338dcd3c24fa9704571ccfcd7cefc0cdb67ece0822a2a",
		},
	},
	"darwin/arm64": {
		URL: fmt.Sprintf("https://dl.k8s.io/release/v%s/bin/darwin/arm64/kubectl", version),
		Digest: schema.Digest{
			Algorithm: "sha256",
			Hex:       "74b5db42979c6ba6c25dfc433509b39221788dc4e0644f218c7549a42405b073",
		},
	},
	"darwin/amd64": {
		URL: fmt.Sprintf("https://dl.k8s.io/release/v%s/bin/darwin/amd64/kubectl", version),
		Digest: schema.Digest{
			Algorithm: "sha256",
			Hex:       "b5980f5a719166ef414455b7f8e9462a3a81c72ef59018cdfca00438af7f3378",
		},
	},
}

type Kubectl string

func EnsureSDK(ctx context.Context) (Kubectl, error) {
	sdk, err := SDK(ctx)
	if err != nil {
		return "", err
	}

	return compute.GetValue(ctx, sdk)
}

func SDK(ctx context.Context) (compute.Computable[Kubectl], error) {
	platform := devhost.RuntimePlatform()
	key := fmt.Sprintf("%s/%s", platform.OS, platform.Architecture)
	ref, ok := Pins[key]
	if !ok {
		return nil, fnerrors.UserError(nil, "platform not supported: %s", key)
	}

	cacheDir, err := dirs.SDKCache("kubectl")
	if err != nil {
		return nil, err
	}

	k3dPath := filepath.Join(cacheDir, "kubectl")
	written := unpack.WriteLocal(k3dPath, 0755, ref)

	return compute.Map(
		tasks.Action("kubectl.ensure").Arg("version", version).HumanReadablef("Ensuring kubectl %s is installed", version),
		compute.Inputs().Computable("kubectl", written),
		compute.Output{},
		func(ctx context.Context, r compute.Resolved) (Kubectl, error) {
			return Kubectl(compute.GetDepValue(r, written, "kubectl")), nil
		}), nil
}

func AllDownloads() []compute.Computable[compute.ByteStream] {
	var downloads []compute.Computable[compute.ByteStream]
	for _, v := range Pins {
		downloads = append(downloads, download.URL(v))
	}
	return downloads
}
