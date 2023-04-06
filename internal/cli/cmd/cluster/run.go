// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
	"namespacelabs.dev/foundation/std/tasks"
	"namespacelabs.dev/go-ids"
)

func newRunCmd() *cobra.Command {
	run := &cobra.Command{
		Use:   "run",
		Short: "Starts a container in an ephemeral environment, optionally exporting ports for public serving.",
		Args:  cobra.NoArgs,
	}

	image := run.Flags().String("image", "", "Which image to run.")
	requestedName := run.Flags().String("name", "", "If no name is specified, one is generated.")
	exportedPorts := run.Flags().Int32SliceP("publish", "p", nil, "Publish the specified ports.")

	run.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		if *image == "" {
			return fnerrors.New("--image is required")
		}

		name := *requestedName
		if name == "" {
			name = generateNameFromImage(*image)
		}

		container := &api.ContainerRequest{
			Name:  name,
			Image: *image,
			Args:  args,
			Flag:  []string{"TERMINATE_ON_EXIT"},
		}

		for _, port := range *exportedPorts {
			container.ExportPort = append(container.ExportPort, &api.ContainerPort{
				Proto: "tcp",
				Port:  port,
			})
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

		fmt.Fprintf(console.Stdout(ctx), "\n  Created new ephemeral environment! (id: %s).\n", resp.ClusterId)
		fmt.Fprintf(console.Stdout(ctx), "\n  More at: %s\n", resp.ClusterUrl)

		for _, ctr := range resp.Container {
			fmt.Fprintf(console.Stdout(ctx), "\n  Running %q\n", ctr.Name)
			if len(ctr.ExportedPort) > 0 {
				fmt.Fprintln(console.Stdout(ctx))
				for _, port := range ctr.ExportedPort {
					fmt.Fprintf(console.Stdout(ctx), "    Exported %d/%s as https://%s\n", port.Port, port.Proto, port.IngressFqdn)
				}
			}
		}

		fmt.Fprintln(console.Stdout(ctx))

		return nil
	})

	return run
}

func generateNameFromImage(image string) string {
	if tag, err := name.NewTag(image); err == nil {
		p := strings.Split(tag.RepositoryStr(), "/")
		last := p[len(p)-1]
		if len(last) < 16 {
			return fmt.Sprintf("%s-%s", last, ids.NewRandomBase32ID(3))
		}
	}

	return ids.NewRandomBase32ID(6)
}
