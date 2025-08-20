// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
	"namespacelabs.dev/foundation/internal/build/buildkit"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/compute/cache"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/executor"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/module"
)

func NewCacheCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cache",
		Short: "Cache related operations (e.g. prune).",
	}

	cmd.AddCommand(newPruneCmd())
	cmd.AddCommand(newTurborepoCmd())

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

func newTurborepoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "turborepo",
		Short: "Turborepo related operations (e.g. list and expire).",
	}

	cmd.AddCommand(newTurborepoListTeamsCmd())
	return cmd
}

func newTurborepoListTeamsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Lists available teams.",
		Args:  cobra.NoArgs,
		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			tokenSrc, err := fnapi.FetchToken(ctx)
			if err != nil {
				return err
			}

			endpoint := turboEndpoint()
			full := fmt.Sprintf("%s/namespace/teams", endpoint)

			req, err := http.NewRequestWithContext(ctx, "GET", full, nil)
			if err != nil {
				return err
			}

			token, err := tokenSrc.IssueToken(ctx, time.Hour, false)
			if err != nil {
				return err
			}

			req.Header.Add("Authorization", "Bearer "+token)
			req.Header.Add("User-Agent", "NamespaceCLI/1.0")

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
			}

			var teams []string
			if err := json.NewDecoder(resp.Body).Decode(&teams); err != nil {
				return fmt.Errorf("decode list teams response: %w", err)
			}

			if len(teams) == 0 {
				fmt.Fprintln(console.Info(ctx), "No turborepo teams found")
				return nil
			}

			fmt.Fprintln(console.Info(ctx), "Available turborepo teams:")
			for _, team := range teams {
				fmt.Fprintln(console.Info(ctx), team)
			}

			return nil
		}),
	}

	return cmd
}

func turboEndpoint() string {
	if endpoint := os.Getenv("NSC_TURBO_ENDPOINT"); endpoint != "" {
		return endpoint
	}

	return "https://turbo.cache.ord.namespaceapis.com"
}
