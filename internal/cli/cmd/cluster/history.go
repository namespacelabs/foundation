// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"encoding/json"

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

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		history := false
		clusters, err := api.ListClusters(ctx, api.Endpoint, history)
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

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		history := true
		clusters, err := api.ListClusters(ctx, api.Endpoint, history)
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
