// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package baseimage

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/integrations/api/compute"
	computev1beta "namespacelabs.dev/integrations/proto/namespace/cloud/compute/v1beta"
)

func newOptimizeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "optimize",
		Short: "Triggers the optimization of a base image.",
		Args:  cobra.NoArgs,
	}

	imageRef := cmd.Flags().String("image_ref", "", "Which image ref to optimize.")
	site := cmd.Flags().String("site", "", "Which site to optimize on. Leave blank for Namespace to decide.")
	pushTag := cmd.Flags().String("push_tag", "", "Publish this tag in the source image's repository after optimization succeeds. The tag points to the source image digest that was optimized.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, specifiedArgs []string) error {
		if *imageRef == "" {
			return fmt.Errorf("--image_ref is required")
		}

		token, err := fnapi.FetchToken(ctx)
		if err != nil {
			return err
		}

		cli, err := compute.NewClient(ctx, token, grpc.WithKeepaliveParams(keepalive.ClientParameters{
			// Keep the connection alive while optimization is ongoing.
			Time:    5 * time.Minute,
			Timeout: 30 * time.Second,
		}))
		if err != nil {
			return err
		}

		c, err := cli.Compute.OptimizeImage(ctx, &computev1beta.OptimizeImageRequest{
			ImageRef: *imageRef,
			Site:     *site,
			PushTag:  *pushTag,
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
