// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"fmt"
	"os"

	toml "github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
)

const pantsCachePathBase = "pantscache"

func NewPantsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pants",
		Short: "Pants-related activities.",
	}

	cache := &cobra.Command{Use: "cache", Short: "Pants cache related functionality."}
	cache.AddCommand(newSetupPantsCacheCmd())

	cmd.AddCommand(cache)

	return cmd
}

func newSetupPantsCacheCmd() *cobra.Command {
	var pantsTomlPath string

	return fncobra.Cmd(&cobra.Command{
		Use:   "setup",
		Short: "Set up a remote Pants cache and generate a Pants toml to use it.",
	}).WithFlags(func(flags *pflag.FlagSet) {
		flags.StringVar(&pantsTomlPath, "pants-toml", "", "If specified, write the toml to this path.")
	}).Do(func(ctx context.Context) error {
		response, err := api.EnsureBazelCache(ctx, api.Methods, "")
		if err != nil {
			return err
		}

		if response.CacheEndpoint == "" {
			return fnerrors.Newf("did not receive a valid cache endpoint")
		}

		globalCfg := map[string]any{
			"remote_cache_read":    true,
			"remote_cache_write":   true,
			"remote_store_address": response.CacheEndpoint,
		}

		if len(response.ServerCaPem) > 0 {
			loc, err := writeTempFile(pantsCachePathBase, "*.cert", []byte(response.ServerCaPem))
			if err != nil {
				return fnerrors.Newf("failed to create temp file: %w", err)
			}

			globalCfg["remote_ca_certs_path"] = loc
		}

		if len(response.ClientCertPem) > 0 {
			loc, err := writeTempFile(pantsCachePathBase, "*.cert", []byte(response.ClientCertPem))
			if err != nil {
				return fnerrors.Newf("failed to create temp file: %w", err)
			}

			globalCfg["remote_client_certs_path"] = loc
		}

		if len(response.ClientKeyPem) > 0 {
			loc, err := writeTempFile(pantsCachePathBase, "*.key", []byte(response.ClientKeyPem))
			if err != nil {
				return fnerrors.Newf("failed to create temp file: %w", err)
			}

			globalCfg["remote_client_key_path"] = loc
		}

		config := map[string]any{
			"GLOBAL": globalCfg,
		}

		serialized, err := toml.Marshal(config)
		if err != nil {
			return fnerrors.Newf("failed to marshal toml: %w", err)
		}

		if pantsTomlPath == "" {
			loc, err := writeTempFile(pantsCachePathBase, "*.toml", serialized)
			if err != nil {
				return fnerrors.Newf("failed to create temp file: %w", err)
			}

			pantsTomlPath = loc
		} else {
			if err := os.WriteFile(pantsTomlPath, serialized, 0644); err != nil {
				return fnerrors.Newf("failed to write %q: %w", pantsTomlPath, err)
			}
		}

		fmt.Fprintf(console.Stdout(ctx), "Wrote Pants toml configuration for remote cache to %s.\n", pantsTomlPath)

		style := colors.Ctx(ctx)
		fmt.Fprintf(console.Stdout(ctx), "\nStart using it by adding:\n")
		fmt.Fprintf(console.Stdout(ctx), "  %s", style.Highlight.Apply(fmt.Sprintf("--pants-config-files=%s\n", pantsTomlPath)))

		return nil
	})
}
