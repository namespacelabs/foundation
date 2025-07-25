// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
	"namespacelabs.dev/foundation/internal/workspace/dirs"
)

const bazelCachePathBase = "bazelcache"

func NewBazelCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bazel",
		Short: "Bazel-related activities.",
	}

	cache := &cobra.Command{Use: "cache", Short: "Bazel cache related functionality."}
	cache.AddCommand(newSetupCacheCmd())

	cmd.AddCommand(cache)

	return cmd
}

func newSetupCacheCmd() *cobra.Command {
	var bazelRcPath, output, certPath string

	return fncobra.Cmd(&cobra.Command{
		Use:   "setup",
		Short: "Set up a remote Bazel cache and generate a bazelrc to use it.",
	}).WithFlags(func(flags *pflag.FlagSet) {
		flags.StringVar(&bazelRcPath, "bazelrc", "", "If specified, write the bazelrc to this path.")
		flags.StringVarP(&output, "output", "o", "plain", "One of plain or json.")

		flags.StringVar(&certPath, "cred_path", "", "If specified, write credentials to this directory. Using this flag also ensures stable file names for all emitted credentials.")
		flags.MarkHidden("cred_path")
	}).Do(func(ctx context.Context) error {
		response, err := api.EnsureBazelCache(ctx, api.Methods, "")
		if err != nil {
			return err
		}

		if response.CacheEndpoint == "" {
			return fnerrors.Newf("did not receive a valid cache endpoint")
		}

		if certPath != "" {
			if err := os.MkdirAll(certPath, 0755); err != nil {
				return fnerrors.Newf("failed to create certificate directory: %w", err)
			}
		}

		out := bazelSetup{
			Endpoint:  response.CacheEndpoint,
			ExpiresAt: response.ExpiresAt,
		}

		if len(response.ServerCaPem) > 0 {
			if certPath == "" {
				loc, err := writeTempFile(bazelCachePathBase, "*.cert", []byte(response.ServerCaPem))
				if err != nil {
					return fnerrors.Newf("failed to create temp file: %w", err)
				}

				out.ServerCaCert = loc
			} else {
				out.ServerCaCert = filepath.Join(certPath, "server_ca.cert")

				if err := writeFile(out.ServerCaCert, []byte(response.ServerCaPem)); err != nil {
					return err
				}
			}
		}

		if len(response.ClientCertPem) > 0 {
			if certPath == "" {
				loc, err := writeTempFile(bazelCachePathBase, "*.cert", []byte(response.ClientCertPem))
				if err != nil {
					return fnerrors.Newf("failed to create temp file: %w", err)
				}

				out.ClientCert = loc
			} else {
				out.ClientCert = filepath.Join(certPath, "client.cert")

				if err := writeFile(out.ClientCert, []byte(response.ClientCertPem)); err != nil {
					return err
				}
			}
		}

		if len(response.ClientKeyPem) > 0 {
			if certPath == "" {
				loc, err := writeTempFile(bazelCachePathBase, "*.key", []byte(response.ClientKeyPem))
				if err != nil {
					return fnerrors.Newf("failed to create temp file: %w", err)
				}

				out.ClientKey = loc
			} else {
				out.ClientKey = filepath.Join(certPath, "client.key")

				if err := writeFile(out.ClientKey, []byte(response.ClientKeyPem)); err != nil {
					return err
				}
			}
		}

		// If set, we always generate a bazelrc file.
		if bazelRcPath != "" {
			data, err := toBazelConfig(out)
			if err != nil {
				return err
			}

			if err := writeFile(bazelRcPath, data); err != nil {
				return err
			}
		}

		switch output {
		case "json":
			d := json.NewEncoder(console.Stdout(ctx))
			d.SetIndent("", "  ")
			if err := d.Encode(out); err != nil {
				return fnerrors.InternalError("failed to encode token as JSON output: %w", err)
			}

		default:
			if output != "" && output != "plain" {
				fmt.Fprintf(console.Warnings(ctx), "unsupported output %q, defaulting to plain\n", output)
			}

			// For plain output, flush the state to a temp bazelrc if none is written yet.
			if bazelRcPath == "" {
				data, err := toBazelConfig(out)
				if err != nil {
					return err
				}

				loc, err := writeTempFile(bazelCachePathBase, "*.bazelrc", data)
				if err != nil {
					return fnerrors.Newf("failed to create temp file: %w", err)
				}

				bazelRcPath = loc
			}

			fmt.Fprintf(console.Stdout(ctx), "Wrote bazelrc configuration for remote cache to %s.\n", bazelRcPath)

			style := colors.Ctx(ctx)
			fmt.Fprintf(console.Stdout(ctx), "\nStart using it by adding:\n")
			fmt.Fprintf(console.Stdout(ctx), "  %s", style.Highlight.Apply(fmt.Sprintf("--bazelrc=%s\n", bazelRcPath)))
		}

		return nil
	})
}

func writeTempFile(base, pattern string, content []byte) (string, error) {
	f, err := dirs.CreateUserTemp(base, pattern)
	if err != nil {
		return "", fnerrors.Newf("failed to create temp file: %w", err)
	}

	if _, err := f.Write(content); err != nil {
		return "", fnerrors.Newf("failed to write to %s: %w", f.Name(), err)
	}

	if err := f.Close(); err != nil {
		return "", fnerrors.Newf("failed to close %s: %w", f.Name(), err)
	}

	return f.Name(), nil
}

func writeFile(path string, content []byte) error {
	if err := os.WriteFile(path, content, 0644); err != nil {
		return fnerrors.Newf("failed to write %q: %w", path, err)
	}

	return nil
}

func toBazelConfig(out bazelSetup) ([]byte, error) {
	var buffer bytes.Buffer
	if _, err := buffer.WriteString(fmt.Sprintf("build --remote_cache=%s\n", out.Endpoint)); err != nil {
		return nil, fnerrors.Newf("failed to append cache endpoint: %w", err)
	}

	if len(out.ServerCaCert) > 0 {
		if _, err := buffer.WriteString(fmt.Sprintf("build --tls_certificate=%s\n", out.ServerCaCert)); err != nil {
			return nil, fnerrors.Newf("failed to append tls_certificate config: %w", err)
		}
	}

	if len(out.ClientCert) > 0 {
		if _, err := buffer.WriteString(fmt.Sprintf("build --tls_client_certificate=%s\n", out.ClientCert)); err != nil {
			return nil, fnerrors.Newf("failed to append tls_client_certificate config: %w", err)
		}
	}

	if len(out.ClientKey) > 0 {
		if _, err := buffer.WriteString(fmt.Sprintf("build --tls_client_key=%s\n", out.ClientKey)); err != nil {
			return nil, fnerrors.Newf("failed to append tls_client_key config: %w", err)
		}
	}

	return buffer.Bytes(), nil
}

type bazelSetup struct {
	Endpoint     string     `json:"endpoint,omitempty"`
	ServerCaCert string     `json:"server_ca_cert,omitempty"`
	ClientCert   string     `json:"client_cert,omitempty"`
	ClientKey    string     `json:"client_key,omitempty"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
}
