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
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/go-ids"
)

func NewDebugShellCmd() *cobra.Command {
	var imageRef string

	cmd := &cobra.Command{
		Use:   "debug-shell",
		Short: "Starts a debug shell in the runtime in the specified environment (e.g. kubernetes cluster).",
		Args:  cobra.NoArgs,
	}

	cmd.Flags().StringVar(&imageRef, "image", imageRef, "If specified, use this image as the basis of the debug shell.")

	return fncobra.CmdWithEnv(cmd, func(ctx context.Context, env provision.Env, args []string) error {
		var imageID oci.ImageID

		if imageRef == "" {
			platforms, err := runtime.For(ctx, env).TargetPlatforms(ctx)
			if err != nil {
				return err
			}

			tag, err := registry.AllocateName(ctx, env, schema.PackageName(env.Workspace().ModuleName+"/debug"), provision.NewBuildID())
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
		} else {
			var err error
			imageID, err = oci.ParseImageID(imageRef)
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
