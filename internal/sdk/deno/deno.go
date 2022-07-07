// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation
package deno

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"namespacelabs.dev/foundation/internal/artifacts"
	"namespacelabs.dev/foundation/internal/artifacts/download"
	"namespacelabs.dev/foundation/internal/artifacts/unpack"
	"namespacelabs.dev/foundation/internal/bytestream"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs/zipfs"
	"namespacelabs.dev/foundation/runtime/rtypes"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/devhost"
	"namespacelabs.dev/foundation/workspace/dirs"
	"namespacelabs.dev/foundation/workspace/tasks"
)

const version = "1.23.3"

var IncludedImports = []string{
	"https://deno.land/std@0.147.0/encoding/base64.ts",
}

var Pins = map[string]artifacts.Reference{
	"linux/amd64": {
		URL: fmt.Sprintf("https://github.com/denoland/deno/releases/download/v%s/deno-x86_64-unknown-linux-gnu.zip", version),
		Digest: schema.Digest{
			Algorithm: "sha256",
			Hex:       "4f6de6e7772dd4cc9f4afcbd583c56a43cd5df2ae38d317c757850fcfcd845cc",
		},
	},
	// "linux/arm64": {
	// 	URL: fmt.Sprintf("https://dl.k8s.io/release/v%s/bin/linux/arm64/kubectl", version),
	// 	Digest: schema.Digest{
	// 		Algorithm: "sha256",
	// 		Hex:       "4be771c8e6a082ba61f0367077f480237f9858ef5efe14b1dbbfc05cd42fc360",
	// 	},
	// },
	"darwin/arm64": {
		URL: fmt.Sprintf("https://github.com/denoland/deno/releases/download/v%s/deno-aarch64-apple-darwin.zip", version),
		Digest: schema.Digest{
			Algorithm: "sha256",
			Hex:       "834ecd6e5530306540bb1c7e2c8a542b36581db1c7388400fbd6578b66347d71",
		},
	},
	"darwin/amd64": {
		URL: fmt.Sprintf("https://github.com/denoland/deno/releases/download/v%s/deno-x86_64-apple-darwin.zip", version),
		Digest: schema.Digest{
			Algorithm: "sha256",
			Hex:       "92008bdee96718abd1e0f21275046347dcccc134f7f982323c6663d3212c315b",
		},
	},
}

type Deno string

func EnsureSDK(ctx context.Context) (Deno, error) {
	sdk, err := SDK(ctx)
	if err != nil {
		return "", err
	}

	return compute.GetValue(ctx, sdk)
}

func SDK(ctx context.Context) (compute.Computable[Deno], error) {
	platform := devhost.RuntimePlatform()
	key := fmt.Sprintf("%s/%s", platform.OS, platform.Architecture)
	ref, ok := Pins[key]
	if !ok {
		return nil, fnerrors.UserError(nil, "platform not supported: %s", key)
	}

	w := unpack.Unpack("deno", zipfs.Unzip(download.URL(ref)))

	return compute.Map(
		tasks.Action("deno.ensure").Arg("version", version),
		compute.Inputs().Computable("deno", w),
		compute.Output{},
		func(ctx context.Context, r compute.Resolved) (Deno, error) {
			denoBin := filepath.Join(compute.MustGetDepValue(r, w, "deno").Files, "deno")

			if err := os.Chmod(denoBin, 0755); err != nil {
				return Deno(denoBin), fnerrors.New("deno: failed to make binary executable: %w", err)
			}

			return Deno(denoBin), nil
		}), nil
}

func AllDownloads() []compute.Computable[bytestream.ByteStream] {
	var downloads []compute.Computable[bytestream.ByteStream]
	for _, v := range Pins {
		downloads = append(downloads, download.URL(v))
	}
	return downloads
}

func (deno Deno) Run(ctx context.Context, dir string, rio rtypes.IO, args ...string) error {
	cacheDir, err := dirs.Cache()
	if err != nil {
		return err
	}

	d, err := schema.DigestOf(IncludedImports)
	if err != nil {
		return err
	}

	denoDir := filepath.Join(cacheDir, "deno", d.Hex)

	cmd := exec.CommandContext(ctx, string(deno), args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), fmt.Sprintf("DENO_DIR=%s", denoDir))
	cmd.Stdin = rio.Stdin
	cmd.Stdout = rio.Stdout
	cmd.Stderr = rio.Stderr

	return cmd.Run()
}

func (deno Deno) CacheImports(ctx context.Context, dir string) error {
	return tasks.Action("deno.cache-imports").Run(ctx, func(ctx context.Context) error {
		out := console.Output(ctx, "deno")

		return deno.Run(ctx, dir, rtypes.IO{Stdout: out, Stderr: out}, append([]string{"cache"}, IncludedImports...)...)
	})
}
