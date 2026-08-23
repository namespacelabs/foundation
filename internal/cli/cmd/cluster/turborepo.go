// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	buildkiteenv "namespacelabs.dev/foundation/internal/buildkite"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	ghenv "namespacelabs.dev/foundation/internal/github/env"
	"namespacelabs.dev/integrations/api"
)

func NewTurborepoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "turborepo",
		Short: "Turborepo cache related functionality.",
	}

	cmd.AddCommand(newTurborepoSetupCmd())
	cmd.AddCommand(newTurborepoListTeamsCmd())
	return cmd
}

func newTurborepoSetupCmd() *cobra.Command {
	var team, tokenFile, output string
	var tokenDur time.Duration
	var readOnly, maskGithubActions, maskBuildkite, maskAuto bool

	cmd := fncobra.Cmd(&cobra.Command{
		Use:   "setup",
		Short: "Set up Turborepo remote caching and output the required environment variables.",
		Args:  cobra.NoArgs,
		Example: `export $(nsc cache turborepo setup --team default)
nsc cache turborepo setup --team default >> "$GITHUB_ENV"`,
	}).WithFlags(func(flags *pflag.FlagSet) {
		flags.StringVar(&team, "team", "", "Turborepo team to use.")
		flags.StringVar(&tokenFile, "token", "", "Use the bearer token stored at this location for authentication instead of the default.")
		fncobra.DurationVar(flags, &tokenDur, "token_duration", 4*time.Hour, "The minimum duration of the configured token.")
		flags.BoolVar(&readOnly, "read-only", false, "Only read from the remote cache. Configures TURBO_CACHE=local:rw,remote:r")
		flags.StringVarP(&output, "output", "o", "plain", "One of plain or json.")
		flags.BoolVar(&maskGithubActions, "mask-github-actions", false, "Mask the token in GitHub Actions.")
		flags.BoolVar(&maskBuildkite, "mask-buildkite", false, "Mask the token in Buildkite.")
		flags.BoolVar(&maskAuto, "mask-auto", false, "Mask the token in a detected CI environment. Supported environments: GitHub Actions and Buildkite.")
	}).Do(func(ctx context.Context) error {
		if strings.TrimSpace(team) == "" {
			return fnerrors.Newf("--team is required")
		}

		var tokenSrc api.TokenSource
		if tokenFile != "" {
			loaded, err := loadTokenFromFile(tokenFile)
			if err != nil {
				return fnerrors.Newf("failed to load token from file: %w", err)
			}
			tokenSrc = loaded
		} else {
			loaded, err := fnapi.FetchToken(ctx)
			if err != nil {
				return err
			}
			tokenSrc = loaded
		}

		token, err := tokenSrc.IssueToken(ctx, tokenDur, false)
		if err != nil {
			return err
		}

		if maskGithubActions || (maskAuto && ghenv.IsRunningInActions()) {
			ghenv.MaskActionsSecretValue(token)
		}
		if maskBuildkite || (maskAuto && buildkiteenv.IsRunningInBuildkite()) {
			if err := buildkiteenv.RedactBuildkiteSecretValue(token); err != nil {
				return err
			}
		}

		out := turborepoSetup{
			API:   turboEndpoint(),
			Team:  team,
			Token: token,
		}
		if readOnly {
			out.Cache = "local:rw,remote:r"
		}

		switch output {
		case "json":
			d := json.NewEncoder(console.Stdout(ctx))
			d.SetIndent("", "  ")
			if err := d.Encode(out); err != nil {
				return fnerrors.InternalError("failed to encode output as JSON: %w", err)
			}

		default:
			if output != "" && output != "plain" {
				fmt.Fprintf(console.Warnings(ctx), "unsupported output %q, defaulting to plain\n", output)
			}

			stdout := console.Stdout(ctx)
			fmt.Fprintf(stdout, "TURBO_API=%s\n", out.API)
			fmt.Fprintf(stdout, "TURBO_TEAM=%s\n", out.Team)
			fmt.Fprintf(stdout, "TURBO_TOKEN=%s\n", out.Token)
			if out.Cache != "" {
				fmt.Fprintf(stdout, "TURBO_CACHE=%s\n", out.Cache)
			}
		}

		return nil
	})

	cmd.MarkFlagsMutuallyExclusive("token", "token_duration")

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

	return "https://turbo.cache.namespaceapi.com"
}

type turborepoSetup struct {
	API   string `json:"api,omitempty"`
	Team  string `json:"team,omitempty"`
	Token string `json:"token,omitempty"`
	Cache string `json:"cache,omitempty"`
}
