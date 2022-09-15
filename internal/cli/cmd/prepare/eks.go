// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package prepare

import (
	"context"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/prepare"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/planning"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/module"
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

			env, err := planning.LoadContext(root, envRef)
			if err != nil {
				return err
			}

			prepares := baseline(env)

			var aws []compute.Computable[[]*schema.DevHost_ConfigureEnvironment]
			aws = append(aws, prepare.PrepareAWSProfile(env, awsProfile))
			aws = append(aws, prepare.PrepareEksCluster(env, clusterName))
			prepares = append(prepares, aws...)
			kube := instantiateKube(env, aws)
			prepares = append(prepares, prepare.PrepareIngress(env, kube))
			return collectPreparesAndUpdateDevhost(ctx, root, prepares)
		}),
	}

	eksCmd.Flags().StringVar(&clusterName, "cluster", "", "The name of the cluster we're configuring.")
	eksCmd.Flags().StringVar(&awsProfile, "aws_profile", awsProfile, "Configures the specified AWS configuration profile.")

	_ = cobra.MarkFlagRequired(eksCmd.Flags(), "cluster")
	_ = cobra.MarkFlagRequired(eksCmd.Flags(), "aws_profile")

	return eksCmd
}
