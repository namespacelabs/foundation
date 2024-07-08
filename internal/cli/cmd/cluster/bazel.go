// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"bytes"
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
	"namespacelabs.dev/foundation/internal/workspace/dirs"
)

func NewBazelCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "bazel",
		Short:  "Bazel-related activities.",
		Hidden: true,
	}

	cache := &cobra.Command{Use: "cache", Short: "Bazel cache related functionality."}
	cache.AddCommand(newSetupCacheCmd())

	cmd.AddCommand(cache)

	return cmd
}

func newSetupCacheCmd() *cobra.Command {
	var outputPath string

	return fncobra.Cmd(&cobra.Command{
		Use:   "setup",
		Short: "Set up a remote Bazel cache and generate a bazelrc to use it.",
	}).WithFlags(func(flags *pflag.FlagSet) {
		flags.StringVar(&outputPath, "output_to", "", "If specified, write the path of the bazelrc to this path.")
	}).Do(func(ctx context.Context) error {
		response, err := api.EnsureBazelCache(ctx, api.Methods)
		if err != nil {
			return err
		}

		if response.CacheEndpoint == "" {
			return fnerrors.New("did not receive a valid cache endpoint")
		}

		var buffer bytes.Buffer
		if _, err := buffer.WriteString(fmt.Sprintf("build --remote_cache=%s\n", response.CacheEndpoint)); err != nil {
			return fnerrors.New("failed to append cache endpoint: %w", err)
		}

		if len(response.ServerCaPem) > 0 {
			loc, err := writeTempFile("*.cert", []byte(response.ServerCaPem))
			if err != nil {
				return fnerrors.New("failed to create temp file: %w", err)
			}

			if _, err := buffer.WriteString(fmt.Sprintf("build --tls_certificate=%s\n", loc)); err != nil {
				return fnerrors.New("failed to append tls_certificate config: %w", err)
			}
		}

		if len(response.ClientCertPem) > 0 {
			loc, err := writeTempFile("*.cert", []byte(response.ClientCertPem))
			if err != nil {
				return fnerrors.New("failed to create temp file: %w", err)
			}

			if _, err := buffer.WriteString(fmt.Sprintf("build --tls_client_certificate=%s\n", loc)); err != nil {
				return fnerrors.New("failed to append tls_client_certificate config: %w", err)
			}
		}

		if len(response.ClientKeyPem) > 0 {
			loc, err := writeTempFile("*.key", []byte(response.ClientKeyPem))
			if err != nil {
				return fnerrors.New("failed to create temp file: %w", err)
			}

			if _, err := buffer.WriteString(fmt.Sprintf("build --tls_client_key=%s\n", loc)); err != nil {
				return fnerrors.New("failed to append tls_client_key config: %w", err)
			}
		}

		loc, err := writeTempFile("*.bazelrc", buffer.Bytes())
		if err != nil {
			return fnerrors.New("failed to create temp file: %w", err)
		}

		if outputPath != "" {
			if err := os.WriteFile(outputPath, []byte(loc), 0644); err != nil {
				return fnerrors.New("failed to write %q: %w", outputPath, err)
			}
		}

		fmt.Fprintf(console.Stdout(ctx), "Wrote bazelrc configuration for remote cache to %s.\n", loc)

		style := colors.Ctx(ctx)
		fmt.Fprintf(console.Stdout(ctx), "\nStart using it by adding:\n")
		fmt.Fprintf(console.Stdout(ctx), "  %s", style.Highlight.Apply(fmt.Sprintf("--bazelrc=%s\n", loc)))

		return nil
	})
}

func writeTempFile(pattern string, content []byte) (string, error) {
	f, err := dirs.CreateUserTemp("bazelcache", pattern)
	if err != nil {
		return "", fnerrors.New("failed to create temp file: %w", err)
	}

	if _, err := f.Write(content); err != nil {
		return "", fnerrors.New("failed to write to %s: %w", f.Name(), err)
	}

	if err := f.Close(); err != nil {
		return "", fnerrors.New("failed to close %s: %w", f.Name(), err)
	}

	return f.Name(), nil
}
