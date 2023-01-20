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
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/mattn/go-zglob"
	"github.com/spf13/cobra"
	"k8s.io/utils/pointer"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/build/assets"
	"namespacelabs.dev/foundation/internal/build/binary"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/internal/runtime/docker"
	"namespacelabs.dev/foundation/internal/runtime/rtypes"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/pkggraph"
)

func NewImagesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "images",
		Short:  "Image related functionality.",
		Hidden: true,
	}

	cmd.AddCommand(unpack())
	cmd.AddCommand(flatten())
	cmd.AddCommand(makeSquash())

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
		var globs []fnfs.HasMatch
		for _, glob := range *extract {
			x, err := zglob.New(glob)
			if err != nil {
				return err
			}
			globs = append(globs, x)
		}

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
			if !matchAny(globs, clean) {
				continue
			}

			switch h.Typeflag {
			case tar.TypeDir:
				if err := dst.MkdirAll(clean, 0700); err != nil {
					return err
				}

			case tar.TypeReg:
				w, err := dst.OpenWrite(clean, h.FileInfo().Mode().Perm())
				if err != nil {
					return err
				}
				_, copyErr := io.Copy(w, tr)
				closeErr := w.Close()
				if copyErr == nil {
					copyErr = closeErr
				}
				if copyErr != nil {
					return copyErr
				}

			default:
				fmt.Fprintf(console.Warnings(ctx), "ignoring %q (%v)\n", clean, h.Typeflag)
			}
		}

		return nil
	})
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

func makeSquash() *cobra.Command {
	makeSquash := &cobra.Command{
		Use:   "mksquash <image-ref> --target <path/to/file>",
		Short: "Flatten an image to a squashfs in the filesystem.",
	}

	image := imageFromArgs(makeSquash)
	target := makeSquash.Flags().String("target", "", "Where the image should be unpacked to.")

	_ = makeSquash.MarkFlagRequired("target")

	return fncobra.With(makeSquash, func(ctx context.Context) error {
		r := mutate.Extract(*image)
		defer r.Close()

		if err := runCommandMaybeNixShell(ctx, rtypes.IO{Stdin: r, Stdout: console.Stdout(ctx), Stderr: console.Stderr(ctx)}, "squashfs-tools-ng", "tar2sqfs", *target); err != nil {
			return err
		}

		return nil
	})
}

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

func resolveImage(ctx context.Context, image string, env cfg.Context, pl *parsing.PackageLoader) (oci.Image, error) {
	if strings.HasPrefix(image, "tar:") {
		return tarball.ImageFromPath(strings.TrimPrefix(image, "tar:"), nil)
	}

	if strings.HasPrefix(image, "binary:") {
		ref, err := schema.StrictParsePackageRef(strings.TrimPrefix(image, "binary:"))
		if err != nil {
			return nil, err
		}

		pkg, err := pl.LoadByName(ctx, ref.AsPackageName())
		if err != nil {
			return nil, err
		}

		bin, err := pkg.LookupBinary(ref.Name)
		if err != nil {
			return nil, err
		}

		prepared, err := binary.PlanBinary(ctx, pl, env, pkg.Location, bin, assets.AvailableBuildAssets{}, binary.BuildImageOpts{})
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

func matchAny(globs []fnfs.HasMatch, path string) bool {
	if len(globs) == 0 {
		return true
	}

	for _, glob := range globs {
		if glob.Match(path) {
			return true
		}
	}
	return false
}

func imageFromArgs(cmd *cobra.Command) *oci.Image {
	env := fncobra.EnvFromValue(cmd, pointer.String("dev"))

	targetImage := new(oci.Image)

	fncobra.PushParse(cmd, func(ctx context.Context, args []string) error {
		if len(args) != 1 {
			return fnerrors.New("expected a single argument, with an image reference")
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
