// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cmd

import (
	"context"
	"io"

	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/fnfs/tarfs"
	"namespacelabs.dev/foundation/runtime/docker"
	"namespacelabs.dev/foundation/workspace/compute"
)

func NewImagesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "images",
		Short:  "Image related functionality.",
		Hidden: true,
	}

	var image, target string
	var insecure bool

	unpack := &cobra.Command{
		Use:   "unpack",
		Short: "Unpack an image to the local filesystem.",

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			platform := docker.HostPlatform()

			img, err := compute.GetValue(ctx, oci.ImageP(image, &platform, insecure))
			if err != nil {
				return err
			}

			fsys := tarfs.FS{
				TarStream: func() (io.ReadCloser, error) {
					return mutate.Extract(img), nil
				},
			}

			return fnfs.CopyTo(ctx, fnfs.ReadWriteLocalFS(target, fnfs.AnnounceWrites(console.Stdout(ctx))), ".", fsys)
		}),
	}

	unpack.Flags().StringVar(&image, "image", "", "Which image to unpack.")
	unpack.Flags().StringVar(&target, "target", "", "Where the image should be unpacked to.")
	unpack.Flags().BoolVar(&insecure, "insecure", false, "Access to the registry is insecure.")

	_ = unpack.MarkFlagRequired("image")
	_ = unpack.MarkFlagRequired("target")

	cmd.AddCommand(unpack)

	return cmd
}
