// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package prepare

import (
	"context"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/parsing/devhost"
	"namespacelabs.dev/foundation/internal/parsing/module"
	"namespacelabs.dev/foundation/internal/prepare"
	"namespacelabs.dev/foundation/internal/providers/nscloud"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
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
	withBuild := cmd.Flags().Bool("with_build_cluster", false, "If true, also configures a build cluster.")

	cmd.RunE = runPrepare(func(ctx context.Context, env cfg.Context) ([]prepare.Stage, error) {
		return []prepare.Stage{prepare.NamespaceCluster(*machineType, *ephemeral, *withBuild)}, nil
	})

	return cmd
}

func newNewBuildClusterCmd() *cobra.Command {
	return fncobra.Cmd(
		&cobra.Command{
			Use:    "new-build-cluster",
			Args:   cobra.NoArgs,
			Hidden: true,
		}).Do(func(ctx context.Context) error {
		root, err := module.FindRoot(ctx, ".")
		if err != nil {
			return err
		}

		env, err := cfg.LoadContext(root, envRef)
		if err != nil {
			return err
		}

		msg, err := nscloud.EnsureBuildCluster(ctx, api.Endpoint)
		if err != nil {
			return err
		}

		c, err := devhost.MakeConfiguration(msg)
		if err != nil {
			return err
		}
		c.Name = env.Environment().Name

		updated, was := devhost.Update(root.LoadedDevHost, c)
		if !was {
			return nil
		}

		return devhost.RewriteWith(ctx, root.ReadWriteFS(), devhost.DevHostFilename, updated)
	})

}
