// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package baseimage

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/integrations/api/compute"
	computev1beta "namespacelabs.dev/integrations/proto/namespace/cloud/compute/v1beta"
)

const maxOptimizeImageRetries = 5

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
			Time:    1 * time.Minute,
			Timeout: 30 * time.Second,
		}))
		if err != nil {
			return err
		}

		err = optimizeImage(ctx, cli.Compute, &computev1beta.OptimizeImageRequest{
			ImageRef: *imageRef,
			Site:     *site,
			PushTag:  *pushTag,
		}, console.Stdout(ctx))
		if err != nil {
			return err
		}

		fmt.Fprintf(console.Stdout(ctx), "\nOptimization complete.\n\n")

		return nil
	})

	return cmd
}

func optimizeImage(ctx context.Context, client computev1beta.ComputeServiceClient, req *computev1beta.OptimizeImageRequest, output io.Writer) error {
	var cursor []byte
	retries := 0

	for {
		var stream grpc.ServerStreamingClient[computev1beta.OptimizeImageProgress]
		var err error
		if len(cursor) == 0 {
			stream, err = client.OptimizeImage(ctx, req)
		} else {
			stream, err = client.WaitOptimizeImage(ctx, &computev1beta.WaitOptimizeImageRequest{OptimizeCursor: cursor})
		}
		if err != nil {
			retries, err = waitToRetryOptimizeImage(ctx, cursor, retries, err)
			if err != nil {
				return err
			}
			continue
		}

		for {
			progress, err := stream.Recv()
			if err == io.EOF {
				return nil
			}
			if err != nil {
				retries, err = waitToRetryOptimizeImage(ctx, cursor, retries, err)
				if err != nil {
					return err
				}
				break
			}

			if len(progress.GetOptimizeCursor()) != 0 {
				cursor = progress.GetOptimizeCursor()
			}
			fmt.Fprintf(output, "Optimization: %s\n", progress.Status.String())
			if progress.GetStatus() == computev1beta.OptimizeImageProgress_DONE {
				return nil
			}
		}
	}
}

func waitToRetryOptimizeImage(ctx context.Context, cursor []byte, retries int, streamErr error) (int, error) {
	if len(cursor) == 0 || !isRetryableOptimizeImageError(streamErr) {
		return retries, streamErr
	}

	retries++
	if retries > maxOptimizeImageRetries {
		return retries, fmt.Errorf("optimize image stream failed after %d retries: %w", retries, streamErr)
	}

	timer := time.NewTimer(time.Duration(retries) * time.Second)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return retries, ctx.Err()
	case <-timer.C:
		return retries, nil
	}
}

func isRetryableOptimizeImageError(err error) bool {
	return status.Code(err) == codes.Unavailable || strings.Contains(err.Error(), "stream terminated by RST_STREAM")
}
