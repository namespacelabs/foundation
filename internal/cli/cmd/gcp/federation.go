// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package gcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
)

const (
	idTokenVersion = 1
	gcpIamUrl      = "https://iam.googleapis.com"
)

func newImpersonateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "impersonate",
		Short:  "To impersonate Namespace workload as a GCP service account.",
		Args:   cobra.NoArgs,
		Hidden: true,
	}

	serviceAccount := cmd.Flags().String("service_account", "", "The GCP service account name to impersonate.")
	workloadIdentityProvider := cmd.Flags().String("workload_identity_provider", "",
		"The full identifier of the GCP Workload Identity Provider, including the project number, pool name, and provider name.")
	credsFile := cmd.Flags().String("write_creds", "", "Instead of outputting, write the credentials to the specified file.")

	duration := fncobra.Duration(cmd.Flags(), "duration", time.Hour, "How long the generated credentials should be valid. Default is 1 hour.")
	gcpTokenLifetime := fncobra.Duration(cmd.Flags(), "gcp_token_lifetime", 10*time.Minute, "Lifetime of GCP tokens generated using these credentials. As long as the credentials are valid the GCP token can be refreshed.")

	return fncobra.Cmd(cmd).Do(func(ctx context.Context) error {
		if *serviceAccount == "" {
			return fnerrors.Newf("--service_account is required")
		}

		if *workloadIdentityProvider == "" {
			return fnerrors.Newf("--workload_identity_provider is required")
		}

		ip := *workloadIdentityProvider
		if strings.HasPrefix(ip, gcpIamUrl) {
			ip = strings.TrimPrefix(ip, gcpIamUrl)
		}

		// Trim leading slashes
		for strings.HasPrefix(ip, "/") {
			ip = strings.TrimPrefix(ip, "/")
		}

		resp, err := fnapi.IssueIdToken(ctx, fmt.Sprintf("%s/%s", gcpIamUrl, ip), idTokenVersion, *duration)
		if err != nil {
			return err
		}

		f, err := os.CreateTemp("", "idtoken")
		if err != nil {
			return err
		}

		if _, err := f.Write([]byte(resp.IdToken)); err != nil {
			return err
		}

		var out = console.Stdout(ctx)
		if *credsFile != "" {
			f, err := os.Create(*credsFile)
			if err != nil {
				return err
			}
			defer f.Close()

			out = f
		}

		// Write a gcloud compatible configuration.
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]any{
			"type":               "external_account",
			"audience":           "//iam.googleapis.com/" + ip,
			"subject_token_type": "urn:ietf:params:oauth:token-type:jwt",
			"token_url":          "https://sts.googleapis.com/v1/token",
			"credential_source": map[string]any{
				"file": f.Name(),
			},
			"service_account_impersonation_url": "https://iamcredentials.googleapis.com/v1/projects/-/serviceAccounts/" + *serviceAccount + ":generateAccessToken",
			"service_account_impersonation": map[string]any{
				"token_lifetime_seconds": int32((*gcpTokenLifetime).Seconds()),
			},
		})
	})
}
