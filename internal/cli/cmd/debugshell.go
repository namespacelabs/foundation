// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cmd

import (
	"context"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/artifacts/registry"
	"namespacelabs.dev/foundation/internal/build/assets"
	"namespacelabs.dev/foundation/internal/build/binary"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/debugshell"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/go-ids"
)

func NewDebugShellCmd() *cobra.Command {
	var imageRef, binaryPackage string

	cmd := &cobra.Command{
		Use:   "debug-shell [--image <image-id>] [--binary_package <path/to/package>]",
		Short: "Starts a debug shell in the runtime in the specified environment (e.g. kubernetes cluster).",
		Args:  cobra.NoArgs,
	}

	cmd.Flags().StringVar(&imageRef, "image", imageRef, "If specified, use this image as the basis of the debug shell.")
	cmd.Flags().StringVar(&binaryPackage, "binary_package", binaryPackage, "If specified, use the resulting image binary as the basis of the debug shell.")

	return fncobra.CmdWithEnv(cmd, func(ctx context.Context, env cfg.Context, args []string) error {
		var imageID oci.ImageID

		cluster, err := runtime.NamespaceFor(ctx, env)
		if err != nil {
			return err
		}

		platforms, err := cluster.Planner().TargetPlatforms(ctx)
		if err != nil {
			return err
		}

		pl := parsing.NewPackageLoader(env)

		loc, err := cwdLoc(env)
		if err != nil {
			return err
		}

		switch {
		case imageRef != "":
			var err error
			imageID, err = oci.ParseImageID(imageRef)
			if err != nil {
				return err
			}

		case binaryPackage != "":

			binaryRef, err := schema.ParsePackageRef(loc.AsPackageName(), binaryPackage)
			if err != nil {
				return err
			}

			pkg, err := pl.LoadByName(ctx, binaryRef.AsPackageName())
			if err != nil {
				return err
			}

			sealedCtx := pkggraph.MakeSealedContext(env, pl.Seal())

			prepared, err := binary.Plan(ctx, pkg, binaryRef.Name, sealedCtx, assets.AvailableBuildAssets{}, binary.BuildImageOpts{Platforms: platforms, UsePrebuilts: true})
			if err != nil {
				return err
			}

			imageID, err = binary.EnsureImage(ctx, sealedCtx, prepared)
			if err != nil {
				return err
			}

		default:
			tag, err := registry.AllocateName(ctx, env, schema.PackageName(env.Workspace().ModuleName()+"/debug"))
			if err != nil {
				return err
			}

			sealedCtx := pkggraph.MakeSealedContext(env, pl.Seal())

			img, err := debugshell.Image(ctx, sealedCtx, platforms, tag)
			if err != nil {
				return err
			}

			imageID, err = compute.GetValue(ctx, img)
			if err != nil {
				return err
			}
		}

		return runtime.RunAttachedStdio(ctx, env, cluster, runtime.DeployableSpec{
			PackageName: loc.AsPackageName(),
			Class:       schema.DeployableClass_ONESHOT,
			Name:        "debug",
			Id:          ids.NewRandomBase32ID(8),
			Attachable:  runtime.AttachableKind_WITH_TTY,
			MainContainer: runtime.ContainerRunOpts{
				Image:   imageID,
				Command: []string{"bash"},
			},
		})
	})
}

func cwdLoc(env cfg.Context) (*fnfs.Location, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	relCwd, err := filepath.Rel(env.Workspace().LoadedFrom().AbsPath, cwd)
	if err != nil {
		return nil, err
	}

	return &fnfs.Location{
		ModuleName: env.Workspace().ModuleName(),
		RelPath:    relCwd,
	}, nil
}
