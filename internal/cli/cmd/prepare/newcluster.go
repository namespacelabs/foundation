// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package prepare

import (
	"context"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/planning"
	"namespacelabs.dev/foundation/internal/prepare"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/module"
)

func newNewClusterCmd() *cobra.Command {
	newClusterCmd := &cobra.Command{
		Use:    "new-cluster",
		Args:   cobra.NoArgs,
		Hidden: true,
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

			var configs []compute.Computable[[]*schema.DevHost_ConfigureEnvironment]
			configs = append(configs, prepare.PrepareNewNamespaceCluster(env))
			prepares = append(prepares, configs...)
			prepares = append(prepares, prepare.PrepareIngress(env, instantiateKube(env, configs)))
			return collectPreparesAndUpdateDevhost(ctx, root, prepares)
		}),
	}

	return newClusterCmd
}
