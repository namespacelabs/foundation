// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package baseimage

import (
	"context"
	"fmt"
	"io"

	computev1beta "buf.build/gen/go/namespace/cloud/protocolbuffers/go/proto/namespace/cloud/compute/v1beta"
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/integrations/api/compute"
	"namespacelabs.dev/integrations/auth"
)

func newOptimizeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "optimize",
		Short: "Triggers the optimization of a base image.",
		Args:  cobra.NoArgs,
	}

	imageRef := cmd.Flags().String("image_ref", "", "Which image ref to optimize.")
	site := cmd.Flags().String("site", "", "Which site to optimize on. Leave blank for Namespace to decide.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, specifiedArgs []string) error {
		if *imageRef == "" {
			return fmt.Errorf("--image_ref is required")
		}

		token, err := auth.LoadDefaults()
		if err != nil {
			return err
		}

		cli, err := compute.NewClient(ctx, token)
		if err != nil {
			return err
		}

		c, err := cli.Compute.OptimizeImage(ctx, &computev1beta.OptimizeImageRequest{
			ImageRef: *imageRef,
			Site:     *site,
		})
		if err != nil {
			return err
		}

		for {
			progress, err := c.Recv()
			if err != nil {
				if err == io.EOF {
					break
				}

				return err
			}

			fmt.Fprintf(console.Stdout(ctx), "Optimization: %s\n", progress.Status.String())

			if progress.GetStatus() == computev1beta.OptimizeImageProgress_DONE {
				break
			}
		}

		fmt.Fprintf(console.Stdout(ctx), "\nOptimization complete.\n\n")

		return nil
	})

	return cmd
}
