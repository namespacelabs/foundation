// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cmd

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/docker/go-units"
	"github.com/dustin/go-humanize"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/daemon"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/spf13/cobra"
	"k8s.io/utils/ptr"
	"namespacelabs.dev/foundation/internal/artifacts"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/build"
	"namespacelabs.dev/foundation/internal/build/binary"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/internal/runtime/docker"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/std/tasks"
)

func NewImagesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "images",
		Short:  "Image related functionality.",
		Hidden: true,
	}

	cmd.AddCommand(unpack())
	cmd.AddCommand(flatten())
	cmd.AddCommand(makeFSImage())

	return cmd
}

func unpack() *cobra.Command {
	unpack := &cobra.Command{
		Use:   "unpack <image-ref> --target <path/to/dir>",
		Short: "Unpack an image to the local filesystem.",
	}

	image := imageFromArgs(unpack)

	target := unpack.Flags().String("target", "", "Where the image should be unpacked to.")
	extract := unpack.Flags().StringArray("extract", nil, "If set, limits the paths being exported to the specified list.")

	_ = unpack.MarkFlagRequired("target")

	return fncobra.With(unpack, func(ctx context.Context) error {
		dst := fnfs.ReadWriteLocalFS(*target, fnfs.AnnounceWrites(console.Stdout(ctx)))

		if err := dst.MkdirAll(".", 0700); err != nil {
			return err
		}

		tr := tar.NewReader(mutate.Extract(*image))
		for {
			h, err := tr.Next()
			if err == io.EOF {
				break
			} else if err != nil {
				return fnerrors.BadInputError("unexpected error: %v", err)
			}

			clean := filepath.Clean(h.Name)
			if !matchAny(*extract, clean) {
				continue
			}

			switch h.Typeflag {
			case tar.TypeDir:
				if err := dst.MkdirAll(clean, 0700); err != nil {
					return err
				}

			case tar.TypeReg:
				if err := dst.MkdirAll(filepath.Dir(clean), 0700); err != nil {
					return err
				}

				var writeErr error
				if h.FileInfo().Size() > 1024*1024 {
					writeErr = tasks.Action("extract").Arg("path", clean).Arg("size", humanize.Bytes(uint64(h.FileInfo().Size()))).
						Run(ctx, func(ctx context.Context) error {
							return writeFile(ctx, dst, clean, tr, h, true)
						})
				} else {
					writeErr = writeFile(ctx, dst, clean, tr, h, false)
				}

				if writeErr != nil {
					return writeErr
				}

			default:
				fmt.Fprintf(console.Warnings(ctx), "ignoring %q (%v)\n", clean, h.Typeflag)
			}
		}

		return nil
	})
}

func writeFile(ctx context.Context, dst fnfs.WriteFS, clean string, tr *tar.Reader, h *tar.Header, progress bool) error {
	w, err := dst.OpenWrite(clean, h.FileInfo().Mode().Perm())
	if err != nil {
		return err
	}

	var r io.Reader
	if progress {
		x := artifacts.NewProgressReader(io.NopCloser(tr), uint64(h.FileInfo().Size()))
		r = x
		tasks.Attachments(ctx).SetProgress(x)
	} else {
		r = tr
	}

	_, copyErr := io.Copy(w, r)
	closeErr := w.Close()
	if copyErr == nil {
		copyErr = closeErr
	}
	if copyErr != nil {
		return copyErr
	}
	return nil
}

func flatten() *cobra.Command {
	flatten := &cobra.Command{
		Use:   "flatten <image-ref> --target <path/to/file>",
		Short: "Flatten an image to a tar file in the filesystem.",
	}

	image := imageFromArgs(flatten)
	target := flatten.Flags().String("target", "", "Where the image should be unpacked to.")

	_ = flatten.MarkFlagRequired("target")

	return fncobra.With(flatten, func(ctx context.Context) error {
		f, err := os.Create(*target)
		if err != nil {
			return err
		}

		r := mutate.Extract(*image)
		defer r.Close()

		if _, err := io.Copy(f, r); err != nil {
			return err
		}

		return f.Close()
	})
}

func makeFSImage() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mkfsimage <image-ref> --kind ext4|squashfs --target <path/to/file>",
		Short: "Flatten an image to a filesystem in the filesystem.",
	}

	image := imageFromArgs(cmd)
	kind := cmd.Flags().String("kind", "ext4", "The filesystem kind.")
	target := cmd.Flags().String("target", "", "Where the image should be unpacked to.")
	size := cmd.Flags().String("size", "0", "The size of the resulting image.")

	_ = cmd.MarkFlagRequired("target")

	return fncobra.With(cmd, func(ctx context.Context) error {
		sz, err := units.FromHumanSize(*size)
		if err != nil {
			return err
		}

		switch strings.ToLower(*kind) {
		case "squashfs", "squash":
			return binary.ToLocalSquashFS(ctx, *image, *target)

		case "ext4fs", "ext4":
			dir, err := os.MkdirTemp("", "ext4")
			if err != nil {
				return err
			}

			defer os.RemoveAll(dir)

			return binary.MakeExt4Image(ctx, *image, dir, *target, sz)
		}

		return fnerrors.BadInputError("make_fs_image: unsupported filesystem %q", *kind)
	})
}

func resolveImage(ctx context.Context, image string, env cfg.Context, pl *parsing.PackageLoader) (oci.Image, error) {
	if strings.HasPrefix(image, "tar:") {
		return tarball.ImageFromPath(strings.TrimPrefix(image, "tar:"), nil)
	}

	if strings.HasPrefix(image, "docker:") {
		n, err := name.ParseReference(strings.TrimPrefix(image, "docker:"))
		if err != nil {
			return nil, err
		}

		return daemon.Image(n, daemon.WithContext(ctx), daemon.WithBufferedOpener())
	}

	if strings.HasPrefix(image, "binary:") {
		ref, err := schema.StrictParsePackageRef(strings.TrimPrefix(image, "binary:"))
		if err != nil {
			return nil, err
		}

		prepared, err := binary.Load(ctx, pl, env, ref, binary.BuildImageOpts{})
		if err != nil {
			return nil, err
		}

		sealedCtx := pkggraph.MakeSealedContext(env, pl.Seal())
		deferred, err := prepared.Image(ctx, sealedCtx)
		if err != nil {
			return nil, err
		}

		resolvable, err := compute.GetValue(ctx, deferred)
		if err != nil {
			return nil, err
		}

		return resolvable.Image()
	}

	insecure := false
	if strings.HasPrefix(image, "insecure:") {
		insecure = true
		image = strings.TrimPrefix(image, "insecure:")
	}

	platform := docker.HostPlatform()

	return compute.GetValue(ctx, oci.ImageP(image, &platform, oci.RegistryAccess{InsecureRegistry: insecure}))
}

func matchAny(files []string, path string) bool {
	if len(files) == 0 {
		return true
	}

	return slices.Contains(files, path)
}

func imageFromArgs(cmd *cobra.Command) *oci.Image {
	env := fncobra.EnvFromValue(cmd, ptr.To("dev"))

	targetImage := new(oci.Image)
	cmd.Flags().Var(build.BuildPlatformsVar{}, "build_platforms", "Which platforms to build the binary for.")

	fncobra.PushParse(cmd, func(ctx context.Context, args []string) error {
		if len(args) != 1 {
			return fnerrors.Newf("expected a single argument, with an image reference")
		}

		image, err := resolveImage(ctx, args[0], *env, parsing.NewPackageLoader(*env))
		if err != nil {
			return err
		}

		*targetImage = image
		return nil
	})

	return targetImage
}
