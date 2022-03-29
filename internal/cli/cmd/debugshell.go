// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cmd

import (
	"context"
	"os"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/artifacts/registry"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/debugshell"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/runtime/rtypes"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/module"
)

func NewDebugShellCmd() *cobra.Command {
	var imageRef string
	envRef := "dev"

	cmd := &cobra.Command{
		Use:   "debug-shell",
		Short: "Starts a debug shell in the runtime in the specified environment (e.g. kubernetes cluster).",
		Args:  cobra.NoArgs,

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			root, err := module.FindRoot(ctx, ".")
			if err != nil {
				return err
			}

			env, err := provision.RequireEnv(root, envRef)
			if err != nil {
				return err
			}

			var imageID oci.ImageID
			if imageRef == "" {
				img, err := debugshell.Image(ctx, env, runtime.For(env).HostPlatforms())
				if err != nil {
					return err
				}

				tag, err := registry.AllocateName(ctx, env, schema.PackageName(root.Workspace.ModuleName+"/debug"), provision.NewBuildID())
				if err != nil {
					return err
				}

				x, err := compute.GetValue(ctx, oci.PublishResolvable(tag, img))
				if err != nil {
					return err
				}

				imageID = x
			} else {
				imageID, err = oci.ParseImageID(imageRef)
				if err != nil {
					return err
				}
			}

			return runtime.For(env).DebugShell(ctx, imageID, rtypes.IO{
				Stdout: os.Stdout,
				Stderr: os.Stderr,
				Stdin:  os.Stdin,
			})
		}),
	}

	cmd.Flags().StringVar(&envRef, "env", envRef, "The environment to provision (as defined in the workspace).")
	cmd.Flags().StringVar(&imageRef, "image", imageRef, "If specified, use this image as the basis of the debug shell.")

	return cmd
}