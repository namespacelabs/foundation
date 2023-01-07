// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package prepare

import (
	"context"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/prepare"
	"namespacelabs.dev/foundation/std/cfg"
)

func newGkeCmd() *cobra.Command {
	var projectId string
	var clusterName string

	// The subcommand `eks` does all of the work done by the parent command in addition to
	// writing the host configuration for the EKS cluster.
	eksCmd := &cobra.Command{
		Use:   "gke --cluster={cluster-name} --env={staging|prod} --project_id={project_id}",
		Short: "Prepares the Elastic Kubernetes Service host config for production.",
		Args:  cobra.NoArgs,
		RunE: runPrepare(func(ctx context.Context, env cfg.Context) ([]prepare.Stage, error) {
			return []prepare.Stage{
				prepare.PrepareGcpProjectID(projectId),
				prepare.PrepareGkeCluster(clusterName),
			}, nil
		}),
	}

	eksCmd.Flags().StringVar(&clusterName, "cluster", "", "The name of the cluster we're configuring.")
	eksCmd.Flags().StringVar(&projectId, "project_id", projectId, "Configures the specified GCP project ID.")

	_ = cobra.MarkFlagRequired(eksCmd.Flags(), "cluster")
	_ = cobra.MarkFlagRequired(eksCmd.Flags(), "project_id")

	return eksCmd
}
