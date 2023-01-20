// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package binary

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/build"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/common"
	"namespacelabs.dev/foundation/internal/runtime/rtypes"
	"namespacelabs.dev/foundation/std/pkggraph"
)

type makeSquashFS struct {
	spec   build.Spec
	target string
}

func (m makeSquashFS) BuildImage(ctx context.Context, env pkggraph.SealedContext, conf build.Configuration) (compute.Computable[oci.Image], error) {
	inner, err := m.spec.BuildImage(ctx, env, conf)
	if err != nil {
		return nil, err
	}

	return compute.Transform("binary.make_squashfs", inner, func(ctx context.Context, img oci.Image) (oci.Image, error) {
		dir, err := os.MkdirTemp("", "squashfs")
		if err != nil {
			return nil, err
		}

		x := filepath.Join(dir, m.target)

		defer os.RemoveAll(dir)

		if err := ToLocalSquashFS(ctx, img, x); err != nil {
			return nil, err
		}

		layer, err := oci.LayerFromFS(ctx, os.DirFS(dir))
		if err != nil {
			return nil, err
		}

		return mutate.AppendLayers(empty.Image, layer)
	}), nil
}

func (m makeSquashFS) PlatformIndependent() bool { return m.spec.PlatformIndependent() }

func runCommandMaybeNixShell(ctx context.Context, io rtypes.IO, pkg, command string, args ...string) error {
	if _, err := exec.LookPath("nix-shell"); err == nil {
		return runNixShell(ctx, io, pkg, command, args)
	}

	loc, err := exec.LookPath(command)
	if err != nil {
		return err
	}

	return runRawCommand(ctx, io, loc, args...)
}

func runRawCommand(ctx context.Context, io rtypes.IO, command string, args ...string) error {
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Stdin = io.Stdin
	cmd.Stdout = io.Stdout
	cmd.Stderr = io.Stderr
	return cmd.Run()
}

func runNixShell(ctx context.Context, io rtypes.IO, pkg, command string, args []string) error {
	return runRawCommand(ctx, io, "nix-shell", "-p", pkg, "--run", strings.Join(append([]string{command}, args...), " "))
}

func ToLocalSquashFS(ctx context.Context, image oci.Image, target string) error {
	r := mutate.Extract(image)
	defer r.Close()

	out := console.TypedOutput(ctx, "tar2sqfs", common.CatOutputTool)

	if err := runCommandMaybeNixShell(ctx, rtypes.IO{Stdin: r, Stdout: out, Stderr: out},
		"squashfs-tools-ng", "tar2sqfs", "-f", target); err != nil {
		return err
	}

	return nil
}
