// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package workspace

import (
	"context"
	"log"

	"github.com/spf13/cobra"
	"google.golang.org/protobuf/reflect/protoreflect"
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

		cidevhost := &schema.DevHost{
			Configure: []*schema.DevHost_ConfigureEnvironment{
				{
					Purpose: schema.Environment_DEVELOPMENT,
					Configuration: wrapAnysOrDie(
						&registry.Provider{Provider: "aws/ecr"},
						&aws.Conf{UseInjectedWebIdentity: true}),
				}, {
					Purpose: schema.Environment_DEVELOPMENT,
					Runtime: "kubernetes",
					Configuration: wrapAnysOrDie(
						&client.HostEnv{Incluster: true}),
				}, {
					Purpose: schema.Environment_PRODUCTION,
					Configuration: wrapAnysOrDie(
						&registry.Provider{Provider: "aws/ecr"},
						&aws.Conf{
							UseInjectedWebIdentity: true,
							AssumeRoleArn:          roleArn,
						}),
				}, {
					Purpose: schema.Environment_PRODUCTION,
					Runtime: "kubernetes",
					Configuration: wrapAnysOrDie(
						&client.HostEnv{Provider: "aws/eks"},
						&eks.EKSCluster{
							Name: clusterName,
							Arn:  clusterArn,
						}),
				}},

			ConfigureTools: wrapAnysOrDie(
				&aws.Conf{UseInjectedWebIdentity: true},
				&registry.Provider{Provider: "aws/ecr"},
				&client.HostEnv{Incluster: true}),

			ConfigurePlatform: []*schema.DevHost_ConfigurePlatform{{
				Configuration: wrapAnysOrDie(
					&buildkit.Overrides{
						BuildkitAddr: *buildkitAddr,
					}),
			}},
		}

		return devhost.RewriteWith(ctx, r, cidevhost)
	})

	return cmd
}

func wrapAnysOrDie(srcs ...protoreflect.ProtoMessage) []*anypb.Any {
	var out []*anypb.Any

	for _, src := range srcs {
		any, err := anypb.New(src)
		if err != nil {
			log.Fatalf("Failed to wrap %s proto in an Any proto: %s", src.ProtoReflect().Descriptor().FullName(), err)
		}
		out = append(out, any)
	}

	return out
}
