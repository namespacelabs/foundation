// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package prepare

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/internal/parsing/devhost"
	"namespacelabs.dev/foundation/internal/prepare"
	"namespacelabs.dev/foundation/internal/protos"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/tasks"
)

var deprecatedConfigs = []string{
	"type.googleapis.com/foundation.build.buildkit.Configuration",
}

var envRef string

func NewPrepareCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "prepare",
		Short: "Prepares the local workspace for development or production.",
		Long: "Prepares the local workspace for development or production.\n\n" +
			"This command will download, create, and run Buildkit and Kubernetes\n" +
			"orchestration containers (conditional on development or production),\n" +
			"in addition to downloading and caching required pre-built images.\n" +
			"Developers will typically run this command only after initializing\n" +
			"the workspace, and it's not a part of the normal refresh-edit\n" +
			"workspace lifecycle.",
		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			return fnerrors.UsageError("For example, you may call `ns prepare local` to configure a local development environment.",
				"One of `local` or `eks` is required.")
		}),
	}

	rootCmd.AddCommand(newEksCmd())
	rootCmd.AddCommand(newLocalCmd())
	rootCmd.AddCommand(newNewClusterCmd())

	rootCmd.PersistentFlags().StringVar(&envRef, "env", "dev", "The environment to access (as defined in the workspace).")

	return rootCmd
}

func downloadPrebuilts(env cfg.Context) compute.Computable[[]oci.ResolvableImage] {
	var prebuilts = []schema.PackageName{}

	return prepare.DownloadPrebuilts(env, prebuilts)
}

func collectPreparesAndUpdateDevhost(ctx context.Context, root *parsing.Root, envName string, prepared compute.Computable[*schema.DevHost_ConfigureEnvironment]) error {
	env, err := cfg.LoadContext(root, "dev")
	if err != nil {
		return err
	}

	x := compute.Map(
		tasks.Action("prepare"),
		compute.Inputs().
			Indigestible("root", root).
			Str("env", envName).
			Computable("prebuilts", downloadPrebuilts(env)).
			Computable("prepared", prepared),
		compute.Output{NotCacheable: true}, func(ctx context.Context, r compute.Resolved) (*schema.DevHost_ConfigureEnvironment, error) {
			results := compute.MustGetDepValue(r, prepared, "prepared")

			prepared := protos.Clone(results)
			prepared.Name = envName

			stdout := console.Stdout(ctx)

			updateCount, err := devHostUpdates(ctx, root, prepared)
			if err != nil {
				return nil, err
			}

			if updateCount == 0 {
				fmt.Fprintln(stdout, "Configuration is up to date, nothing to do.")
				return nil, nil
			}

			if err := devhost.RewriteWith(ctx, root.ReadWriteFS(), devhost.DevHostFilename, root.LoadedDevHost); err != nil {
				return nil, err
			}

			return prepared, nil
		})

	if _, err := compute.GetValue(ctx, x); err != nil {
		return err
	}

	return nil

}

func devHostUpdates(ctx context.Context, root *parsing.Root, confs ...*schema.DevHost_ConfigureEnvironment) (int, error) {
	var updateCount int
	updated, was := devhost.Update(root.LoadedDevHost, confs...)
	if was {
		updateCount++
	}

	// Make sure that the subsequent calls observe an up to date configuration.
	// XXX this is not right, Root() should be immutable.
	root.LoadedDevHost = updated

	// Remove deprecated bits.
	for k, u := range root.LoadedDevHost.Configure {
		var without []*anypb.Any

		for _, cfg := range u.Configuration {
			if !slices.Contains(deprecatedConfigs, cfg.TypeUrl) {
				without = append(without, cfg)
			} else {
				updateCount++
			}
		}

		if len(without) == 0 {
			root.LoadedDevHost.Configure[k] = nil // Mark for removal.
		} else {
			u.Configuration = without
		}
	}

	k := 0
	for {
		if k >= len(root.LoadedDevHost.Configure) {
			break
		}

		if root.LoadedDevHost.Configure[k] == nil {
			root.LoadedDevHost.Configure = append(root.LoadedDevHost.Configure[:k], root.LoadedDevHost.Configure[k+1:]...)
			updateCount++
		} else {
			k++
		}
	}

	return updateCount, nil
}
