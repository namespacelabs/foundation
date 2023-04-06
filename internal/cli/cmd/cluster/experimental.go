// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
	"namespacelabs.dev/foundation/std/tasks"
	"namespacelabs.dev/go-ids"
)

func newExperimentalCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "experimental",
		Args:   cobra.NoArgs,
		Hidden: true,
	}

	run := &cobra.Command{
		Use: "run",
	}

	image := run.Flags().String("image", "", "Which image to run.")
	requestedName := run.Flags().String("name", "", "If no name is specified, one is generated.")

	run.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		if *image == "" {
			return fnerrors.New("--image is required")
		}

		name := *requestedName
		if name == "" {
			name = ids.NewRandomBase32ID(6)
		}

		container := &api.ContainerRequest{
			Name:  name,
			Image: *image,
			Args:  args,
			Flag:  []string{"TERMINATE_ON_EXIT"},
		}

		resp, err := tasks.Return(ctx, tasks.Action("nscloud.create-containers"), func(ctx context.Context) (*api.CreateContainersResponse, error) {
			var response api.CreateContainersResponse
			if err := api.Endpoint.CreateContainers.Do(ctx, api.CreateContainersRequest{
				Container: []*api.ContainerRequest{container},
			}, fnapi.DecodeJSONResponse(&response)); err != nil {
				return nil, err
			}
			return &response, nil
		})
		if err != nil {
			return err
		}

		if _, err := api.WaitCluster(ctx, api.Endpoint, resp.ClusterId, api.WaitClusterOpts{}); err != nil {
			return err
		}

		return nil
	})

	cmd.AddCommand(run)

	return cmd
}
