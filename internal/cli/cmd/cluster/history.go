// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"encoding/json"
	"time"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
)

func NewListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Lists all of your clusters.",
		Args:  cobra.NoArgs,
	}

	output := cmd.Flags().StringP("output", "o", "plain", "One of plain or json.")
	labels := cmd.Flags().StringToString("label", nil, "Constrain list to the specified labels.")
	all := cmd.Flags().Bool("all", false, "If true, returl all clusters, not just manually created ones.")

	cmd.Flags().MarkHidden("label")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		history := false
		clusters, err := api.ListClusters(ctx, api.Methods, api.ListOpts{
			PreviousRuns: history,
			Labels:       *labels,
			All:          *all,
		})
		if err != nil {
			return err
		}

		if *output == "json" {
			stdout := console.Stdout(ctx)
			enc := json.NewEncoder(stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(clusters.Clusters)
		}
		if len(clusters.Clusters) == 0 {
			printCreateClusterMsg(ctx)
			return nil
		}
		return staticTableClusters(ctx, clusters.Clusters, history)
	})

	return cmd
}

func newHistoryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "history",
		Short: "History of your previous running clusters.",
		Args:  cobra.NoArgs,
	}

	output := cmd.Flags().StringP("output", "o", "plain", "One of plain or json.")
	labels := cmd.Flags().StringToString("label", nil, "Constrain list to the specified labels.")
	since := cmd.Flags().Duration("since", time.Hour*24*7, "Contrain list to selected duration.")
	all := cmd.Flags().Bool("all", false, "If true, returl all clusters, not just manually created ones.")

	cmd.Flags().MarkHidden("label")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		history := true
		startTs := time.Now().Add(-*since)
		clusters, err := api.ListClusters(ctx, api.Methods, api.ListOpts{
			PreviousRuns: history,
			NotOlderThan: &startTs,
			Labels:       *labels,
			All:          *all,
		})
		if err != nil {
			return err
		}

		if *output == "json" {
			stdout := console.Stdout(ctx)
			enc := json.NewEncoder(stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(clusters.Clusters)
		}

		if len(clusters.Clusters) == 0 {
			printCreateClusterMsg(ctx)
			return nil
		}

		return staticTableClusters(ctx, clusters.Clusters, history)
	})

	return cmd
}
