// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cmd

import (
	"context"
	"os"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/build/binary"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/artifacts/registry"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/debugshell"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/go-ids"
)

func NewDebugShellCmd() *cobra.Command {
	var imageRef, binaryPackage string

	cmd := &cobra.Command{
		Use:   "debug-shell",
		Short: "Starts a debug shell in the runtime in the specified environment (e.g. kubernetes cluster).",
		Args:  cobra.NoArgs,
	}

	cmd.Flags().StringVar(&imageRef, "image", imageRef, "If specified, use this image as the basis of the debug shell.")
	cmd.Flags().StringVar(&binaryPackage, "binary_package", binaryPackage, "If specified, use the resulting image binary as the basis of the debug shell.")

	return fncobra.CmdWithEnv(cmd, func(ctx context.Context, env provision.Env, args []string) error {
		var imageID oci.ImageID

		platforms, err := runtime.For(ctx, env).TargetPlatforms(ctx)
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
			pkg, err := workspace.NewPackageLoader(env.Root()).LoadByName(ctx, schema.PackageName(binaryPackage))
			if err != nil {
				return err
			}

			prepared, err := binary.Plan(ctx, pkg, binary.BuildImageOpts{Platforms: platforms, UsePrebuilts: true})
			if err != nil {
				return err
			}

			imageID, err = binary.EnsureImage(ctx, env, prepared)
			if err != nil {
				return err
			}

		default:
			tag, err := registry.AllocateName(ctx, env, schema.PackageName(env.Workspace().ModuleName+"/debug"))
			if err != nil {
				return err
			}

			img, err := debugshell.Image(ctx, env, platforms, tag)
			if err != nil {
				return err
			}

			imageID, err = compute.GetValue(ctx, img)
			if err != nil {
				return err
			}
		}

		return runtime.For(ctx, env).RunAttached(ctx, "debug-"+ids.NewRandomBase32ID(8), runtime.ServerRunOpts{
			Image:   imageID,
			Command: []string{"bash"},
		}, runtime.TerminalIO{
			TTY:    true,
			Stdout: os.Stdout,
			Stderr: os.Stderr,
			Stdin:  os.Stdin,
		})
	})
}
