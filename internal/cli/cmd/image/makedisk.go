// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package image

import (
	"context"
	"fmt"

	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/artifacts/registry"
	"namespacelabs.dev/foundation/internal/build/binary"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/internal/parsing/platform"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/module"
	"namespacelabs.dev/foundation/std/pkggraph"
)

func newMakeDiskCmd() *cobra.Command {
	var (
		insecure bool
		imageRef string
		target   string
		size     int64
		envBound string
		buildRef string
	)

	cmd := &cobra.Command{
		Use:  "make-disk",
		Args: cobra.NoArgs,

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			root, err := module.FindRoot(ctx, ".")
			if err != nil {
				return err
			}

			env, err := cfg.LoadContext(root, envBound)
			if err != nil {
				return err
			}

			if size == 0 {
				return fnerrors.New("size is required")
			}

			var platforms []specs.Platform
			for _, plat := range []string{"linux/amd64", "linux/arm64"} {
				p, err := platform.ParsePlatform(plat)
				if err != nil {
					return err
				}

				platforms = append(platforms, p)
			}

			imgid, err := oci.ParseImageID(imageRef)
			if err != nil {
				return err
			}

			image, err := makeImage(ctx, env, imgid, buildRef, insecure, platforms)
			if err != nil {
				return err
			}

			var imgwithplat []oci.ImageWithPlatform
			for _, plat := range platforms {
				plat := plat // Close plat.
				x := binary.MakeDisk(compute.Transform("get-image", image, func(ctx context.Context, r oci.ResolvableImage) (oci.Image, error) {
					return r.ImageForPlatform(plat)
				}), target, size, true)

				imgwithplat = append(imgwithplat, oci.ImageWithPlatform{
					Image:    oci.MakeNamedImage(platform.FormatPlatform(plat), x),
					Platform: plat,
				})
			}

			repository := registry.StaticRepository(nil, imgid.Repository, oci.RegistryAccess{InsecureRegistry: insecure})

			published := oci.PublishResolvable(repository, oci.MakeImageIndex(imgwithplat...), nil)

			res, err := compute.Get(ctx, published)
			if err != nil {
				return err
			}

			fmt.Fprintf(console.Stdout(ctx), "%s\n", res.Value.ImageRef())

			return nil
		}),
	}

	cmd.Flags().BoolVar(&insecure, "insecure", false, "Set to true to access registries over insecure communications.")
	cmd.Flags().StringVar(&imageRef, "image", "", "The image to convert.")
	cmd.Flags().StringVar(&buildRef, "build", "", "The image to build.")
	cmd.Flags().StringVar(&target, "target", "disk.ext4.zstd", "The name of the disk.")
	cmd.Flags().Int64Var(&size, "size", 0, "The size.")
	cmd.Flags().StringVar(&envBound, "env", "dev", "The environment.")

	return cmd
}

func makeImage(ctx context.Context, env cfg.Context, imgid oci.ImageID, buildRef string, insecure bool, platforms []specs.Platform) (compute.Computable[oci.ResolvableImage], error) {
	if buildRef != "" {
		pkgRef, err := schema.StrictParsePackageRef(buildRef)
		if err != nil {
			return nil, err
		}

		pl := parsing.NewPackageLoader(env)

		bin, err := binary.Load(ctx, pl, env, pkgRef, binary.BuildImageOpts{
			Platforms: platforms,
		})
		if err != nil {
			return nil, err
		}

		return bin.Image(ctx, pkggraph.MakeSealedContext(env, pl.Seal()))
	}

	return oci.Prebuilt(imgid, oci.RegistryAccess{InsecureRegistry: insecure}), nil
}
