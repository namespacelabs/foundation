// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	goproxyv1beta "namespacelabs.dev/integrations/proto/namespace/private/goproxy"
)

const goProxyTokenDuration = time.Hour

func NewGoProxyCacheCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "goproxy",
		Short: "Go module proxy related functionality.",
	}

	cmd.AddCommand(newSetupGoProxyCmd())
	cmd.AddCommand(newGoProxyHandleNetrcCmd())

	return cmd
}

func newSetupGoProxyCmd() *cobra.Command {
	var envFile string

	return fncobra.Cmd(&cobra.Command{
		Use:   "setup",
		Short: "Start a Go module proxy and write an env file that configures Go to use it.",
		Long: `Start a Go module proxy and write an env file that configures Go to use it.

The proxy serves public modules only; other modules fall back to direct.
The path of the env file is printed to stdout, so it can be sourced with:

  source "$(nsc cache goproxy setup)"`,
		Args: cobra.NoArgs,
	}).WithFlags(func(flags *pflag.FlagSet) {
		flags.StringVar(&envFile, "envfile", "", "Write the env file to this path instead of a temporary file.")
	}).Do(func(ctx context.Context) error {
		cli, err := fnapi.NewGoProxyManagerServiceClient(ctx)
		if err != nil {
			return err
		}

		stream, err := cli.EnsureGoProxy(ctx, connect.NewRequest(&goproxyv1beta.EnsureGoProxyRequest{}))
		if err != nil {
			return fnerrors.Newf("failed to start the go module proxy: %w", err)
		}
		defer stream.Close()

		var proxyURL string
		for stream.Receive() {
			if url := stream.Msg().GetProxy().GetProxyUrl(); url != "" {
				proxyURL = url
			}
		}
		if err := stream.Err(); err != nil {
			return fnerrors.Newf("failed to start the go module proxy: %w", err)
		}
		if proxyURL == "" {
			return fnerrors.Newf("did not receive a go module proxy endpoint")
		}

		self, err := os.Executable()
		if err != nil {
			return fnerrors.Newf("failed to resolve the nsc binary: %w", err)
		}

		// GOAUTH replaces the default "netrc" mode, so keep the user's existing authentication ahead of ours.
		goauth := os.Getenv("GOAUTH")
		if goauth == "" || goauth == "off" {
			goauth = "netrc"
		}
		goauth += fmt.Sprintf(";%s cache goproxy handle-netrc --proxy_url %s", self, proxyURL)

		var contents strings.Builder
		for _, kv := range [][2]string{
			{"GOPROXY", proxyURL + ",direct"},
			{"GOAUTH", goauth},
		} {
			fmt.Fprintf(&contents, "export %s=%s\n", kv[0], shellQuote(kv[1]))
		}

		path, err := writeGoProxyEnvFile(envFile, contents.String())
		if err != nil {
			return err
		}

		fmt.Fprintln(console.Stdout(ctx), path)
		return nil
	})
}

func writeGoProxyEnvFile(path, contents string) (string, error) {
	if path != "" {
		if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
			return "", fnerrors.Newf("failed to write env file: %w", err)
		}
		return path, nil
	}

	f, err := os.CreateTemp("", "nsc-goproxy-*.env")
	if err != nil {
		return "", fnerrors.Newf("failed to create env file: %w", err)
	}

	if _, err := f.WriteString(contents); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return "", fnerrors.Newf("failed to write env file: %w", err)
	}

	if err := f.Close(); err != nil {
		_ = os.Remove(f.Name())
		return "", fnerrors.Newf("failed to write env file: %w", err)
	}

	return f.Name(), nil
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// newGoProxyHandleNetrcCmd implements the GOAUTH command protocol (see `go help goauth`).
// Go runs it with no arguments, and again with the failing URL as an argument after a 4xx response.
func newGoProxyHandleNetrcCmd() *cobra.Command {
	var proxyURL string

	return fncobra.Cmd(&cobra.Command{
		Use:    "handle-netrc [url]",
		Short:  "Supply Go module proxy credentials to the go command (GOAUTH).",
		Args:   cobra.MaximumNArgs(1),
		Hidden: true,
	}).WithFlags(func(flags *pflag.FlagSet) {
		flags.StringVar(&proxyURL, "proxy_url", "", "The Go module proxy URL to supply credentials for.")
		_ = cobra.MarkFlagRequired(flags, "proxy_url")
	}).Do(func(ctx context.Context) error {
		done := console.EnterInputMode(ctx)
		defer done()

		token, err := fnapi.IssueToken(ctx, goProxyTokenDuration)
		if err != nil {
			return err
		}

		_, err = fmt.Fprintf(os.Stdout, "%s\n\nX-NSC-Ingress-Auth: Bearer %s\n\n", strings.TrimSuffix(proxyURL, "/"), token)
		return err
	})
}
