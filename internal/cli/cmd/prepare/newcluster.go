// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package prepare

import (
	"context"

	"github.com/spf13/cobra"
	"k8s.io/utils/strings/slices"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/prepare"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
)

func newNewClusterCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "new-cluster",
		Args:   cobra.NoArgs,
		Hidden: true,
	}

	machineType := cmd.Flags().String("machine_type", "", "Specify the machine type.")
	ephemeral := cmd.Flags().Bool("ephemeral", false, "Create an ephemeral cluster.")
	features := cmd.Flags().StringSlice("features", nil, "A set of features to create the cluster with.")
	withBuild := cmd.Flags().Bool("with_build_cluster", false, "If true, also configures a build cluster.")

	cmd.RunE = runPrepare(func(ctx context.Context, env cfg.Context) (compute.Computable[*schema.DevHost_ConfigureEnvironment], error) {
		actualFeatures := slices.Clone(*features)
		if *withBuild {
			actualFeatures = append(actualFeatures, "BUILD_CLUSTER")
		}

		return prepare.PrepareCluster(env, prepare.PrepareNewNamespaceCluster(env, *machineType, *ephemeral, actualFeatures)), nil
	})

	return cmd
}
