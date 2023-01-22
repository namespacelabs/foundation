// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package binary

import (
	"context"
	"io"
	"os"
	"path/filepath"

	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"namespacelabs.dev/foundation/framework/rpcerrors/multierr"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/build"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/common"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/runtime/rtypes"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/std/tasks"
)

type makeExt4Image struct {
	spec   build.Spec
	target string
	size   int64
}

func (m makeExt4Image) BuildImage(ctx context.Context, env pkggraph.SealedContext, conf build.Configuration) (compute.Computable[oci.Image], error) {
	if m.size == 0 {
		return nil, fnerrors.BadInputError("size must be specified")
	}

	inner, err := m.spec.BuildImage(ctx, env, conf)
	if err != nil {
		return nil, err
	}

	return compute.Transform("binary.make-ext4-image", inner, func(ctx context.Context, img oci.Image) (oci.Image, error) {
		dir, err := os.MkdirTemp("", "ext4")
		if err != nil {
			return nil, err
		}

		defer os.RemoveAll(dir)

		mount := filepath.Join(dir, "mount")
		if err := os.Mkdir(mount, 0755); err != nil {
			return nil, err
		}

		out := filepath.Join(dir, "out")
		x := filepath.Join(out, m.target)
		if err := os.MkdirAll(filepath.Dir(x), 0755); err != nil {
			return nil, err
		}

		if err := toExt4Image(ctx, dir, img, x, m.size); err != nil {
			return nil, err
		}

		layer, err := oci.LayerFromFS(ctx, os.DirFS(out))
		if err != nil {
			return nil, err
		}

		return mutate.AppendLayers(empty.Image, layer)
	}), nil
}

func (m makeExt4Image) PlatformIndependent() bool { return m.spec.PlatformIndependent() }

func toExt4Image(ctx context.Context, dir string, image oci.Image, target string, size int64) error {
	tmpFile := filepath.Join(dir, "image.tar")
	if err := writeFile(ctx, tmpFile, image); err != nil {
		return err
	}

	f, err := os.Create(tmpFile)
	if err != nil {
		return err
	}

	if err := f.Close(); err != nil {
		return err
	}

	if err := os.Truncate(tmpFile, size); err != nil {
		return err
	}

	out := console.TypedOutput(ctx, "write-ext4-image", common.CatOutputTool)
	io := rtypes.IO{Stdout: out, Stderr: out}

	if err := runCommandMaybeNixShell(ctx, io, "e2fsprogs", "mkfs.ext4", target); err != nil {
		return err
	}

	mount := filepath.Join(dir, "mount")
	if err := runRawCommand(ctx, io, "mount", "-o", "loop", target, mount); err != nil {
		return err
	}

	tarErr := runRawCommand(ctx, io, "tar", "xf", tmpFile, "-C", mount)
	umountErr := runRawCommand(ctx, io, "umount", mount)

	return multierr.New(tarErr, umountErr)
}

func writeFile(ctx context.Context, filepath string, image oci.Image) error {
	return tasks.Action("binary.make-ext4-image.write-image-as-tar").Run(ctx, func(ctx context.Context) error {
		f, err := os.Create(filepath)
		if err != nil {
			return err
		}

		r := mutate.Extract(image)
		_, copyErr := io.Copy(f, r)
		rErr := r.Close()
		fErr := f.Close()

		return multierr.New(copyErr, rErr, fErr)
	})
}
