// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package runs

import (
	"context"
	"io/ioutil"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
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
	var u imageUploader
	u.SetupFlags(cmd, flags)

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		image, err := v.ComputeImage(ctx, args[0])
		if err != nil {
			return err
		}

		return u.PublishAndWrite(ctx, image)
	})

	return cmd
}

type imageUploader struct {
	EnvName       string
	WorkspacePath string
	Repository    string
	Output        string
}

func (v *imageUploader) SetupFlags(cmd *cobra.Command, flags *pflag.FlagSet) {
	flags.StringVar(&v.EnvName, "env", "dev", "The environment to load configuration from.")
	flags.StringVar(&v.WorkspacePath, "workspace", "", "The workspace where to load configuration from.")
	flags.StringVar(&v.Repository, "repository", "", "The repository to upload to.")
	flags.StringVar(&v.Output, "output", "", "Where to write the image reference to.")

	_ = cmd.MarkFlagRequired("workspace")
	_ = cmd.MarkFlagRequired("repository")
	_ = cmd.MarkFlagRequired("output")
}

func (v *imageUploader) Publish(ctx context.Context, img oci.NamedImage) (oci.ImageID, error) {
	devhost.HasRuntime = func(_ string) bool { return true } // We never really use the runtime, so just assert that any runtime exists.

	workspace.ModuleLoader = cuefrontend.ModuleLoader
	workspace.MakeFrontend = cuefrontend.NewFrontend

	root, err := module.FindRoot(ctx, v.WorkspacePath)
	if err != nil {
		return oci.ImageID{}, err
	}

	env, err := provision.RequireEnv(root, v.EnvName)
	if err != nil {
		return oci.ImageID{}, err
	}

	tag, err := registry.RawAllocateName(ctx, devhost.ConfigKeyFromEnvironment(env), v.Repository)
	if err != nil {
		return oci.ImageID{}, fnerrors.InternalError("failed to allocate image for stored results: %w", err)
	}

	imageID, err := compute.GetValue(ctx, oci.PublishImage(tag, img).ImageID())
	if err != nil {
		return oci.ImageID{}, fnerrors.InternalError("failed to store results: %w", err)
	}

	return imageID, nil
}

func (v *imageUploader) PublishAndWrite(ctx context.Context, img oci.NamedImage) error {
	imageID, err := v.Publish(ctx, img)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(v.Output, []byte(imageID.ImageRef()), 0644)
}
