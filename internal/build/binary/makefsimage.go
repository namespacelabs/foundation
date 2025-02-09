// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package binary

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/klauspost/compress/zstd"
	"namespacelabs.dev/foundation/framework/rpcerrors/multierr"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/build"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/runtime/docker"
	"namespacelabs.dev/foundation/internal/runtime/rtypes"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/std/tasks"
)

type makeExt4Image struct {
	spec   build.Spec
	target string
	size   int64
	raw    bool
}

func validateExt4(size int64) error {
	if size == 0 {
		return fnerrors.BadInputError("size must be specified")
	}

	return nil
}

func (m makeExt4Image) BuildImage(ctx context.Context, env pkggraph.SealedContext, conf build.Configuration) (compute.Computable[oci.Image], error) {
	inner, err := m.spec.BuildImage(ctx, env, conf)
	if err != nil {
		return nil, err
	}

	if err := validateExt4(m.size); err != nil {
		return nil, err
	}

	return MakeDisk(inner, m.target, m.size, m.raw), nil
}

func (m makeExt4Image) Description() string { return fmt.Sprintf("makeExt4(%s)", m.spec.Description()) }

func MakeDisk(inner compute.Computable[oci.Image], target string, size int64, raw bool) compute.Computable[oci.Image] {
	return compute.Transform("binary.make-ext4-image", inner, func(ctx context.Context, img oci.Image) (oci.Image, error) {
		dir, err := os.MkdirTemp("", "ext4")
		if err != nil {
			return nil, err
		}

		compute.On(ctx).Cleanup(tasks.Action("remove tmpdir"), func(ctx context.Context) error {
			return os.RemoveAll(dir)
		})

		tmpDir := filepath.Join(dir, "out")
		target := filepath.Join(tmpDir, target)
		if err := MakeExt4Image(ctx, img, dir, target, size); err != nil {
			return nil, err
		}

		if raw {
			comp := target + ".zstd"
			outf, err := os.Create(comp)
			if err != nil {
				return nil, err
			}

			defer outf.Close()

			enc, err := zstd.NewWriter(outf)
			if err != nil {
				return nil, err
			}

			src, err := os.Open(target)
			if err != nil {
				return nil, err
			}

			if _, err := io.Copy(enc, src); err != nil {
				return nil, err
			}

			if err := enc.Close(); err != nil {
				return nil, err
			}

			if err := outf.Close(); err != nil {
				return nil, err
			}

			layer, err := oci.RawZstdLayerFrom(comp, "application/x-namespace-raw-zstd-ext4")
			if err != nil {
				return nil, err
			}

			return mutate.AppendLayers(empty.Image, layer)
		}

		layer, err := oci.LayerFromFS(ctx, os.DirFS(tmpDir))
		if err != nil {
			return nil, err
		}

		return mutate.AppendLayers(empty.Image, layer)
	})
}

func (m makeExt4Image) PlatformIndependent() bool { return m.spec.PlatformIndependent() }

func toExt4Image(ctx context.Context, tmpdir string, image oci.Image, target string, size int64) error {
	tmpFile := filepath.Join(tmpdir, "image.tar")
	if err := flattenToFile(ctx, tmpFile, image); err != nil {
		return err
	}

	f, err := os.Create(target)
	if err != nil {
		return err
	}

	if err := f.Close(); err != nil {
		return err
	}

	if err := os.Truncate(target, size); err != nil {
		return err
	}

	// nsdev build-binary internal/build/binary/imageutil --base_repository=registry.eu-services.namespace.systems --build_platforms linux/arm64,linux/amd64
	img, err := oci.ParseImageID("registry.eu-services.namespace.systems/namespacelabs.dev/foundation/internal/build/binary/imageutil@sha256:afbded41542118d6b535735bbe692ca8d5fabd8e139edbc43014d4345dc25563")
	if err != nil {
		return err
	}

	v, err := oci.FetchRemoteImage(ctx, img, oci.RegistryAccess{PublicImage: true})
	if err != nil {
		return err
	}

	h, _ := image.Digest()

	out := console.Output(ctx, "make-ext4-image "+h.Hex)

	var run rtypes.RunToolOpts
	run.Privileged = true
	run.IO.Stdout = out
	run.IO.Stderr = out
	run.WorkingDir = "/"
	run.Image = v
	run.Command = []string{"/bake", "-source", "/source", "-target", "/target"}
	run.Mounts = append(run.Mounts,
		&rtypes.LocalMapping{
			HostPath:      target,
			ContainerPath: "/target",
		},
		&rtypes.LocalMapping{
			HostPath:      tmpFile,
			ContainerPath: "/source",
		},
	)

	return docker.Runtime().Run(ctx, run)
}

func flattenToFile(ctx context.Context, filepath string, image oci.Image) error {
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

func MakeExt4Image(ctx context.Context, image oci.Image, tmpdir, target string, size int64) error {
	if err := validateExt4(size); err != nil {
		return err
	}

	mount := filepath.Join(tmpdir, "mount")

	if err := os.Mkdir(mount, 0755); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}

	if err := toExt4Image(ctx, tmpdir, image, target, size); err != nil {
		return err
	}

	return nil
}
