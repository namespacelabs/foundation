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
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
)

func NewListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Lists all of your instances.",
		Args:  cobra.NoArgs,
	}

	output := cmd.Flags().StringP("output", "o", "plain", "One of plain or json.")
	labels := cmd.Flags().StringToString("label", nil, "Constrain list to the specified labels.")
	all := cmd.Flags().Bool("all", false, "If true, returl all instances, not just manually created ones.")

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
			if err := enc.Encode(transform(clusters.Clusters)); err != nil {
				return fnerrors.InternalError("failed to encode instance list as JSON output: %w", err)
			}

			return nil
		}

		if len(clusters.Clusters) == 0 {
			PrintCreateClusterMsg(ctx)
			return nil
		}

		return staticTableClusters(ctx, clusters.Clusters, history)
	})

	return cmd
}

func transform(mds []api.KubernetesClusterMetadata) []map[string]any {
	var x []map[string]any
	for _, md := range mds {
		x = append(x, transformForOutput(md))
	}
	return x
}

// We keep a validated set of output fields which should not change.
func transformForOutput(md api.KubernetesClusterMetadata) map[string]any {
	m := map[string]any{
		"cluster_id":     md.ClusterId,
		"created_at":     md.Created,
		"shape":          md.Shape,
		"ingress_domain": md.IngressDomain,
		"labels":         md.UserLabel,
	}

	if md.DestroyedAt != "" {
		m["destroyed_at"] = md.DestroyedAt
	}

	if len(md.Platform) == 1 {
		m["platform"] = md.Platform[0]
	}

	if md.CreatorId != "" {
		m["creator_id"] = md.CreatorId
	}

	return m
}

func newHistoryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "history",
		Short: "History of your previous running instances.",
		Args:  cobra.NoArgs,
	}

	output := cmd.Flags().StringP("output", "o", "plain", "One of plain or json.")
	labels := cmd.Flags().StringToString("label", nil, "Constrain list to the specified labels.")
	since := cmd.Flags().Duration("since", time.Hour*24*7, "Contrain list to selected duration.")
	all := cmd.Flags().Bool("all", false, "If true, returl all instances, not just manually created ones.")
	maxEntries := cmd.Flags().Int64("max_entries", 100, "Maximum number of instances to return.")

	cmd.Flags().MarkHidden("label")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		history := true
		startTs := time.Now().Add(-*since)
		clusters, err := api.ListClusters(ctx, api.Methods, api.ListOpts{
			PreviousRuns: history,
			NotOlderThan: &startTs,
			Labels:       *labels,
			All:          *all,
			MaxEntries:   *maxEntries,
		})
		if err != nil {
			return err
		}

		if *output == "json" {
			stdout := console.Stdout(ctx)
			enc := json.NewEncoder(stdout)
			enc.SetIndent("", "  ")
			if err := enc.Encode(clusters.Clusters); err != nil {
				return fnerrors.InternalError("failed to encode instance history as JSON output: %w", err)
			}

			return nil
		}

		if len(clusters.Clusters) == 0 {
			PrintCreateClusterMsg(ctx)
			return nil
		}

		return staticTableClusters(ctx, clusters.Clusters, history)
	})

	return cmd
}
