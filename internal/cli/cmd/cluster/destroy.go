// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console/tui"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
)

func newDestroyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "destroy [cluster-id]",
		Short: "Destroys an existing cluster.",
		Args:  cobra.ArbitraryArgs,
	}

	force := cmd.Flags().Bool("force", false, "Skip the confirmation step.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		clusters, err := selectClusters(ctx, args)
		if err != nil {
			if errors.Is(err, ErrEmptyClusteList) {
				printCreateClusterMsg(ctx)
				return nil
			}
			return err
		}

		for _, cluster := range clusters {
			if !*force {
				result, err := tui.Ask(ctx, "Do you want to remove this cluster?",
					fmt.Sprintf(`This is a destructive action.

	Type %q for it to be removed.`, cluster.ClusterId), "")
				if err != nil {
					return err
				}

				if result != cluster.ClusterId {
					return context.Canceled
				}
			}

			if err := api.DestroyCluster(ctx, api.Endpoint, cluster.ClusterId); err != nil {
				return err
			}
		}

		return nil
	})

	return cmd
}
