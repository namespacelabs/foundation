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
		clusterIDs := args
		if len(clusterIDs) == 0 {
			selected, err := selectClusterID(ctx)
			if err != nil {
				if errors.Is(err, ErrEmptyClusteList) {
					printCreateClusterMsg(ctx)
					return nil
				}
				return err
			}
			if selected == "" {
				return nil
			}
			clusterIDs = []string{selected}
		}

		for _, cluster := range clusterIDs {
			if !*force {
				result, err := tui.Ask(ctx, "Do you want to remove this cluster?",
					fmt.Sprintf(`This is a destructive action.

	Type %q for it to be removed.`, cluster), "")
				if err != nil {
					return err
				}

				if result != cluster {
					return context.Canceled
				}
			}

			if err := api.DestroyCluster(ctx, api.Endpoint, cluster); err != nil {
				return err
			}
		}

		return nil
	})

	return cmd
}
