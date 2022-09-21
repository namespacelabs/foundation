// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package prepare

import (
	"context"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/prepare"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/std/planning"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/module"
)

func newNewClusterCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "new-cluster",
		Args:   cobra.NoArgs,
		Hidden: true,
	}

	machineType := cmd.Flags().String("machine_type", "", "Specify the machine type.")
	ephemeral := cmd.Flags().Bool("ephemeral", false, "Create an ephemeral cluster.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		root, err := module.FindRoot(ctx, ".")
		if err != nil {
			return err
		}

		env, err := planning.LoadContext(root, envRef)
		if err != nil {
			return err
		}

		sealedCtx := pkggraph.MakeSealedContext(env, workspace.NewPackageLoader(env).Seal())

		prepares := baseline(sealedCtx)
		prepares = append(prepares, prepare.PrepareCluster(env, prepare.PrepareNewNamespaceCluster(env, *machineType, *ephemeral))...)
		return collectPreparesAndUpdateDevhost(ctx, root, prepares)
	})

	return cmd
}
