// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package workspace

import (
	"context"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/build/buildkit"
	"namespacelabs.dev/foundation/build/registry"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/parsing/devhost"
	"namespacelabs.dev/foundation/internal/protos"
	"namespacelabs.dev/foundation/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/schema"
	aws "namespacelabs.dev/foundation/universe/aws/configuration"
	"namespacelabs.dev/foundation/universe/aws/eks"
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
		cidevhost := &schema.DevHost{
			Configure: []*schema.DevHost_ConfigureEnvironment{
				{
					Purpose: schema.Environment_DEVELOPMENT,
					Configuration: protos.WrapAnysOrDie(
						&registry.Provider{Provider: "aws/ecr"},
						&aws.Configuration{UseInjectedWebIdentity: true}),
				}, {
					Purpose: schema.Environment_DEVELOPMENT,
					Runtime: "kubernetes",
					Configuration: protos.WrapAnysOrDie(
						&client.HostEnv{Incluster: true}),
				}, {
					Purpose: schema.Environment_PRODUCTION,
					Configuration: protos.WrapAnysOrDie(
						&registry.Provider{Provider: "aws/ecr"},
						&aws.Configuration{
							UseInjectedWebIdentity: true,
							AssumeRoleArn:          roleArn,
						}),
				}, {
					Purpose: schema.Environment_PRODUCTION,
					Runtime: "kubernetes",
					Configuration: protos.WrapAnysOrDie(
						&client.HostEnv{Provider: "aws/eks"},
						&eks.EKSCluster{
							Name: clusterName,
							Arn:  clusterArn,
						}),
				}},

			ConfigureTools: protos.WrapAnysOrDie(
				&aws.Configuration{UseInjectedWebIdentity: true},
				&registry.Provider{Provider: "aws/ecr"},
				&client.HostEnv{Incluster: true}),

			ConfigurePlatform: []*schema.DevHost_ConfigurePlatform{{
				Configuration: protos.WrapAnysOrDie(
					&buildkit.Overrides{
						BuildkitAddr: *buildkitAddr,
					}),
			}},
		}

		return devhost.RewriteWith(ctx, fnfs.ReadWriteLocalFS(*workspaceDir), devhost.DevHostFilename, cidevhost)
	})

	return cmd
}
