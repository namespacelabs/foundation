// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
package deno

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/internal/artifacts"
	"namespacelabs.dev/foundation/internal/artifacts/download"
	"namespacelabs.dev/foundation/internal/artifacts/unpack"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs/zipfs"
	"namespacelabs.dev/foundation/internal/runtime/rtypes"
	"namespacelabs.dev/foundation/internal/workspace/dirs"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/tasks"
)

const version = "1.25.4"

var IncludedImports = []string{
	"https://deno.land/std@0.147.0/encoding/base64.ts",
}

var Pins = map[string]artifacts.Reference{
	"linux/amd64": {
		URL: fmt.Sprintf("https://github.com/denoland/deno/releases/download/v%s/deno-x86_64-unknown-linux-gnu.zip", version),
		Digest: schema.Digest{
			Algorithm: "sha256",
			Hex:       "ee29ceabab5141ce56ffbb4cbb74a9662de325e5b336933d19058764ea13633d",
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
			Hex:       "c29526fd6835e65505efc07d7d372943f418bc7d97d172ef86e4d86e1e42ca69",
		},
	},
	"darwin/amd64": {
		URL: fmt.Sprintf("https://github.com/denoland/deno/releases/download/v%s/deno-x86_64-apple-darwin.zip", version),
		Digest: schema.Digest{
			Algorithm: "sha256",
			Hex:       "2dd4700707baaf29548ab72d3fddd994d50f65f8c46c7044fdd4c0e6b4a94f78",
		},
	},
}

type Deno string

func EnsureSDK(ctx context.Context, p specs.Platform) (Deno, error) {
	sdk, err := SDK(ctx, p)
	if err != nil {
		return "", err
	}

	return compute.GetValue(ctx, sdk)
}

func SDK(ctx context.Context, p specs.Platform) (compute.Computable[Deno], error) {
	key := fmt.Sprintf("%s/%s", p.OS, p.Architecture)
	ref, ok := Pins[key]
	if !ok {
		return nil, fnerrors.UserError(nil, "platform not supported: %s", key)
	}

	w := unpack.Unpack("deno", zipfs.Unzip(download.URL(ref)))

	return compute.Map(
		tasks.Action("deno.ensure").Arg("version", version),
		compute.Inputs().Computable("deno", w).JSON("platform", p),
		compute.Output{},
		func(ctx context.Context, r compute.Resolved) (Deno, error) {
			denoBin := filepath.Join(compute.MustGetDepValue(r, w, "deno").Files, "deno")

			if err := os.Chmod(denoBin, 0755); err != nil {
				return Deno(denoBin), fnerrors.New("deno: failed to make binary executable: %w", err)
			}

			return Deno(denoBin), nil
		}), nil
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
