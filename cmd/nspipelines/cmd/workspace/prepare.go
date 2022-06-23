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
	"namespacelabs.dev/foundation/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/devhost"
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

		ciregistry, err := anypb.New(&registry.Provider{
			Provider: "aws/ecr",
		})
		if err != nil {
			return err
		}

		ciruntime, err := anypb.New(&client.HostEnv{
			Incluster: true,
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
			Configure: []*schema.DevHost_ConfigureEnvironment{
				{
					Configuration: []*anypb.Any{ciregistry},
				},
				{
					Runtime:       "kubernetes",
					Configuration: []*anypb.Any{ciruntime},
				},
			},
			ConfigureTools: []*anypb.Any{ciregistry, ciruntime},
			ConfigurePlatform: []*schema.DevHost_ConfigurePlatform{{
				Configuration: []*anypb.Any{cibuildkit},
			}},
		}

		return devhost.RewriteWith(ctx, r, cidevhost)
	})

	return cmd
}
