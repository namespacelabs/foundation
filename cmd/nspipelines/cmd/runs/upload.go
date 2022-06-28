// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package runs

import (
	"context"
	"io/ioutil"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/artifacts/registry"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/frontend/cuefrontend"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/devhost"
	"namespacelabs.dev/foundation/workspace/module"
)

func newUploadCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:  "upload",
		Args: cobra.ExactArgs(1),
	}

	flags := cmd.Flags()

	var v resultImage
	v.SetupFlags(cmd, flags)

	envName := flags.String("env", "dev", "The environment to load configuration from.")
	workspacePath := flags.String("workspace", "", "The workspace where to load configuration from.")
	output := flags.String("output", "", "Where to write the image reference to.")
	repository := flags.String("repository", "", "The repository to upload to.")

	_ = cmd.MarkFlagRequired("workspace")
	_ = cmd.MarkFlagRequired("output")
	_ = cmd.MarkFlagRequired("repository")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		devhost.HasRuntime = func(_ string) bool { return true } // We never really use the runtime, so just assert that any runtime exists.

		workspace.ModuleLoader = cuefrontend.ModuleLoader
		workspace.MakeFrontend = cuefrontend.NewFrontend

		root, err := module.FindRoot(ctx, *workspacePath)
		if err != nil {
			return err
		}

		env, err := provision.RequireEnv(root, *envName)
		if err != nil {
			return err
		}

		image, err := v.ComputeImage(ctx, args[0])
		if err != nil {
			return err
		}

		tag, err := registry.RawAllocateName(ctx, devhost.ConfigKeyFromEnvironment(env), *repository)
		if err != nil {
			return fnerrors.InternalError("failed to allocate image for stored results: %w", err)
		}

		imageID, err := compute.GetValue(ctx, oci.PublishImage(tag, image))
		if err != nil {
			return fnerrors.InternalError("failed to store results: %w", err)
		}

		return ioutil.WriteFile(*output, []byte(imageID.ImageRef()), 0644)
	})

	return cmd
}
