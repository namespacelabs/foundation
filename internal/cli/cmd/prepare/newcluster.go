// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package prepare

import (
	"context"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/internal/parsing/module"
	"namespacelabs.dev/foundation/internal/prepare"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/pkggraph"
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

		env, err := cfg.LoadContext(root, envRef)
		if err != nil {
			return err
		}

		sealedCtx := pkggraph.MakeSealedContext(env, parsing.NewPackageLoader(env).Seal())

		prepares := baseline(sealedCtx)
		prepares = append(prepares, prepare.PrepareCluster(env, prepare.PrepareNewNamespaceCluster(env, *machineType, *ephemeral))...)
		return collectPreparesAndUpdateDevhost(ctx, root, prepares)
	})

	return cmd
}
