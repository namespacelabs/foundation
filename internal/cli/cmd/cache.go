// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cmd

import (
	"context"

	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
	"namespacelabs.dev/foundation/internal/build/buildkit"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/compute/cache"
	"namespacelabs.dev/foundation/internal/executor"
	"namespacelabs.dev/foundation/internal/parsing/module"
	"namespacelabs.dev/foundation/std/cfg"
)

func NewCacheCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cache",
		Short: "Cache related operations (e.g. prune).",
	}

	cmd.AddCommand(newPruneCmd())

	return cmd
}

func newPruneCmd() *cobra.Command {
	what := []string{"foundation", "buildkit"}

	cmd := &cobra.Command{
		Use:   "prune",
		Short: "Remove all foundation-managed caches.",
		Args:  cobra.NoArgs,

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			root, err := module.FindRoot(ctx, ".")
			if err != nil {
				return err
			}

			eg := executor.New(ctx, "fn.prune")

			if slices.Contains(what, "foundation") {
				eg.Go(func(ctx context.Context) error {
					return cache.Prune(ctx)
				})
			}

			if slices.Contains(what, "buildkit") {
				eg.Go(func(ctx context.Context) error {
					// XXX make platform configurable.
					return buildkit.Prune(ctx, cfg.MakeConfigurationWith("prune", root.Workspace(), cfg.ConfigurationSlice{
						PlatformConfiguration: root.DevHost().ConfigurePlatform,
					}), nil)
				})
			}

			// XXX remove go caches?
			return eg.Wait()
		}),
	}

	cmd.Flags().StringArrayVar(&what, "caches", what, "Which caches to prune. List of: foundation, buildkit.")

	return cmd
}
