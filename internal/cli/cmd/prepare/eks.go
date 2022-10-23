// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package prepare

import (
	"context"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/parsing/module"
	"namespacelabs.dev/foundation/internal/prepare"
	"namespacelabs.dev/foundation/std/cfg"
)

func newEksCmd() *cobra.Command {
	var awsProfile string
	var clusterName string

	// The subcommand `eks` does all of the work done by the parent command in addition to
	// writing the host configuration for the EKS cluster.
	eksCmd := &cobra.Command{
		Use:   "eks --cluster={cluster-name} --env={staging|prod} --aws_profile={profile}",
		Short: "Prepares the Elastic Kubernetes Service host config for production.",
		Args:  cobra.NoArgs,
		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			root, err := module.FindRoot(ctx, ".")
			if err != nil {
				return err
			}

			env, err := cfg.LoadContext(root, envRef)
			if err != nil {
				return err
			}

			return collectPreparesAndUpdateDevhost(ctx, root, envRef,
				prepare.PrepareCluster(env,
					prepare.PrepareAWSProfile(awsProfile),
					prepare.PrepareEksCluster(clusterName),
				))
		}),
	}

	eksCmd.Flags().StringVar(&clusterName, "cluster", "", "The name of the cluster we're configuring.")
	eksCmd.Flags().StringVar(&awsProfile, "aws_profile", awsProfile, "Configures the specified AWS configuration profile.")

	_ = cobra.MarkFlagRequired(eksCmd.Flags(), "cluster")
	_ = cobra.MarkFlagRequired(eksCmd.Flags(), "aws_profile")

	return eksCmd
}
