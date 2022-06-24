// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package workspace

import (
	"context"

	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/build/buildkit"
	"namespacelabs.dev/foundation/build/registry"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/providers/aws"
	"namespacelabs.dev/foundation/providers/aws/eks"
	"namespacelabs.dev/foundation/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/devhost"
)

// TODO make this configurable per workspace
const (
	roleArn     = "arn:aws:iam::846205600055:role/namespace-ci"
	clusterName = "montblanc"
	clusterArn  = "arn:aws:eks:us-east-2:846205600055:cluster/" + clusterName
)

func newPrepareCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "prepare",
		Short: "Prepare the workspace for deployment in from a foundation pipeline.",
	}

	flag := cmd.Flags()
	workspaceDir := flag.String("workspace", ".", "The workspace directory to parse.")
	buildkitAddr := flag.String("buildkit_address", "tcp://buildkitd:1234", "The buildkit address to configure.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		r := workspace.NewRoot(*workspaceDir)
		if err := devhost.Prepare(ctx, r); err != nil {
			return err
		}

		ecr, err := anypb.New(&registry.Provider{
			Provider: "aws/ecr",
		})
		if err != nil {
			return err
		}

		incluster, err := anypb.New(&client.HostEnv{
			Incluster: true,
		})
		if err != nil {
			return err
		}

		staging, err := anypb.New(&aws.Conf{
			AssumeRoleArn: roleArn,
		})
		if err != nil {
			return err
		}
		eksProvider, err := anypb.New(&client.HostEnv{
			Provider: "aws/eks",
		})
		if err != nil {
			return err
		}
		eksCluster, err := anypb.New(&eks.EKSCluster{
			Name: clusterName,
			Arn:  clusterArn,
		})
		if err != nil {
			return err
		}

		cibuildkit, err := anypb.New(&buildkit.Overrides{
			BuildkitAddr: *buildkitAddr,
		})
		if err != nil {
			return err
		}

		cidevhost := &schema.DevHost{
			Configure: []*schema.DevHost_ConfigureEnvironment{{
				Configuration: []*anypb.Any{ecr},
			}, {
				Purpose:       schema.Environment_DEVELOPMENT,
				Runtime:       "kubernetes",
				Configuration: []*anypb.Any{incluster},
			}, {
				Purpose:       schema.Environment_PRODUCTION,
				Configuration: []*anypb.Any{staging},
			}, {
				Purpose:       schema.Environment_PRODUCTION,
				Runtime:       "kubernetes",
				Configuration: []*anypb.Any{eksProvider, eksCluster},
			}},
			ConfigureTools: []*anypb.Any{ecr, incluster},
			ConfigurePlatform: []*schema.DevHost_ConfigurePlatform{{
				Configuration: []*anypb.Any{cibuildkit},
			}},
		}

		if err := devhost.RewriteWith(ctx, r, cidevhost); err != nil {
			return err
		}

	})

	return cmd
}
