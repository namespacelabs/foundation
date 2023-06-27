// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package auth

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnapi"
)

const gcpIamUrl = "https://iam.googleapis.com"

func NewIssueIdTokenCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "issue-id-token",
		Short:  "Generate a Namespace ID token to authenticate with cloud providers.",
		Args:   cobra.NoArgs,
		Hidden: true,
	}

	workloadProvider := cmd.Flags().String("workload_identity_provider", "", "The full identifier of the Workload Identity Provider, including the project number, pool name, and provider name.")
	output := cmd.Flags().StringP("output", "o", "plain", "One of plain or json.")
	return fncobra.Cmd(cmd).Do(func(ctx context.Context) error {
		if *workloadProvider == "" {
			return fmt.Errorf("workload identity provider is not provided")
		}

		resp, err := fnapi.IssueIdToken(ctx, fmt.Sprintf("%s/%s", gcpIamUrl, *workloadProvider))
		if err != nil {
			return err
		}

		return printResult(ctx, *output, resp)
	})
}

func printResult(ctx context.Context, output string, resp fnapi.IssueIdTokenResponse) error {
	switch output {
	case "json":
		d := json.NewEncoder(console.Stdout(ctx))
		d.SetIndent("", "  ")
		return d.Encode(resp)

	default:
		if output != "" && output != "plain" {
			fmt.Fprintf(console.Warnings(ctx), "unsupported output %q, defaulting to plain\n", output)
		}

		fmt.Fprintf(console.Stdout(ctx), "ID token: %s\n", resp.IdToken)
		fmt.Fprintln(console.Stdout(ctx))
	}

	return nil
}
