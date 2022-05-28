// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubectl

import (
	"context"
	"fmt"
	"path/filepath"

	"namespacelabs.dev/foundation/internal/artifacts/download"
	"namespacelabs.dev/foundation/internal/artifacts/unpack"
	"namespacelabs.dev/foundation/internal/bytestream"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/devhost"
	"namespacelabs.dev/foundation/workspace/tasks"
)

const version = "1.24.0"
const algorithm = "sha256"

type Pin struct {
	url, digestUrl string
}

type Kubectl string

var supported = []string{"linux/amd64", "linux/arm64", "darwin/amd64", "darwin/arm64"}

var Pins = map[string]Pin{}

func init() {
	for _, s := range supported {
		Pins[s] = Pin{
			url:       fmt.Sprintf("https://dl.k8s.io/release/v%s/bin/%s/kubectl", version, s),
			digestUrl: fmt.Sprintf("https://dl.k8s.io/release/v%s/bin/%s/kubectl.sha256", version, s),
		}
	}
}

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

	fetch := download.UrlAndDigest(ref.url, ref.digestUrl, algorithm)
	w := unpack.Unpack("kubectl", unpack.MakeFilesystemForContents("kubectl", 0755, ref.url, fetch))

	return compute.Map(
		tasks.Action("kubectl.ensure").Arg("version", version).HumanReadablef("Ensuring kubectl %s is installed", version),
		compute.Inputs().Computable("kubectl", w),
		compute.Output{},
		func(ctx context.Context, r compute.Resolved) (Kubectl, error) {
			return Kubectl(filepath.Join(compute.MustGetDepValue(r, w, "kubectl").Files, "kubectl")), nil
		}), nil
}

func AllDownloads() []compute.Computable[bytestream.ByteStream] {
	var downloads []compute.Computable[bytestream.ByteStream]
	for _, v := range Pins {
		download := download.UrlAndDigest(v.url, v.digestUrl, algorithm)
		downloads = append(downloads, download)
	}
	return downloads
}
