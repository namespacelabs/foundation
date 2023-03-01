package cluster

import (
	"context"
	"encoding/json"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
)

func newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Lists all of your clusters.",
		Args:  cobra.NoArgs,
	}

	rawOutput := cmd.Flags().Bool("raw_output", false, "Dump the resulting server response, without formatting.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		history := false
		clusters, err := api.ListClusters(ctx, api.Endpoint, history)
		if err != nil {
			return err
		}

		if *rawOutput {
			stdout := console.Stdout(ctx)
			enc := json.NewEncoder(stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(clusters)
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

	rawOutput := cmd.Flags().Bool("raw_output", false, "Dump the resulting server response, without formatting.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		history := true
		clusters, err := api.ListClusters(ctx, api.Endpoint, history)
		if err != nil {
			return err
		}

		if *rawOutput {
			stdout := console.Stdout(ctx)
			enc := json.NewEncoder(stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(clusters)
		}

		if len(clusters.Clusters) == 0 {
			printCreateClusterMsg(ctx)
			return nil
		}

		return staticTableClusters(ctx, clusters.Clusters, history)
	})

	return cmd
}
