// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package prepare

import (
	"context"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/networking/ingress/nginx"
	"namespacelabs.dev/foundation/internal/prepare"
	"namespacelabs.dev/foundation/std/cfg"
)

func newGkeCmd() *cobra.Command {
	var projectId string
	var clusterName string
	var experimentalGCLB bool
	var ingressClass string

	// The subcommand `gke` does all of the work done by the parent command in addition to
	// writing the host configuration for the GKE cluster.
	gkeCmd := &cobra.Command{
		Use:   "gke --cluster={cluster-name} --env={staging|prod} --project_id={project_id}",
		Short: "Prepares the Elastic Kubernetes Service host config for production.",
		Args:  cobra.NoArgs,
		RunE: runPrepare(func(ctx context.Context, env cfg.Context) ([]prepare.Stage, error) {
			if experimentalGCLB {
				if ingressClass != "" {
					return nil, fnerrors.Newf("--experimental_use_gclb can't be used with --ingress_class")
				}

				ingressClass = "gclb"
			}

			if ingressClass == "" {
				ingressClass = nginx.IngressClass().Name()
			}

			return []prepare.Stage{
				prepare.PrepareGcpProjectID(projectId),
				prepare.PrepareGkeCluster(clusterName, ingressClass),
			}, nil
		}),
	}

	gkeCmd.Flags().StringVar(&clusterName, "cluster", "", "The name of the cluster we're configuring.")
	gkeCmd.Flags().StringVar(&projectId, "project_id", projectId, "Configures the specified GCP project ID.")
	gkeCmd.Flags().BoolVar(&experimentalGCLB, "experimental_use_gclb", experimentalGCLB, "Use GCLB with GKE, rather than an incluster nginx ingress.")
	gkeCmd.Flags().StringVar(&ingressClass, "ingress_class", "", "Specify the ingress class.")

	_ = cobra.MarkFlagRequired(gkeCmd.Flags(), "cluster")
	_ = cobra.MarkFlagRequired(gkeCmd.Flags(), "project_id")

	return gkeCmd
}
