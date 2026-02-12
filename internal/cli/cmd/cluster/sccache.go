// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"buf.build/gen/go/namespace/cloud/connectrpc/go/proto/namespace/cloud/integrations/httpcache/v1beta/httpcachev1betaconnect"
	iamv1beta "buf.build/gen/go/namespace/cloud/protocolbuffers/go/proto/namespace/cloud/iam/v1beta"
	httpcachev1beta "buf.build/gen/go/namespace/cloud/protocolbuffers/go/proto/namespace/cloud/integrations/httpcache/v1beta"
	"connectrpc.com/connect"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"google.golang.org/protobuf/types/known/timestamppb"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/go-ids"
	"namespacelabs.dev/integrations/api/iam"
	"namespacelabs.dev/integrations/auth"
)

func NewSccacheCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sccache",
		Short: "sccache cache related functionality.",
	}

	cmd.AddCommand(newSetupSccacheCacheCmd())
	cmd.AddCommand(newCreateSccacheTokenCmd())

	return cmd
}

func newSetupSccacheCacheCmd() *cobra.Command {
	var output string
	var name, site string
	var tokenFile string

	return fncobra.Cmd(&cobra.Command{
		Use:   "setup",
		Short: "Set up a remote sccache cache and output the required environment variables.",
		Long: `Set up a remote sccache cache and output the required environment variables.

This command provisions a remote build cache and outputs the environment
variables needed to configure sccache to use it.

The output includes:
  SCCACHE_WEBDAV_ENDPOINT - The WebDAV endpoint URL
  SCCACHE_WEBDAV_KEY_PREFIX - The key prefix path
  SCCACHE_WEBDAV_TOKEN - The authentication token`,
	}).WithFlags(func(flags *pflag.FlagSet) {
		flags.StringVarP(&output, "output", "o", "plain", "One of plain or json.")
		flags.StringVar(&name, "cache_name", "", "A name for the cache.")
		flags.StringVar(&site, "site", "", "Site preference (e.g., 'iad', 'fra'). If not set, determined automatically.")
		flags.StringVar(&tokenFile, "token", "", "Use the bearer token stored at this location for authentication instead of the default.")
	}).Do(func(ctx context.Context) error {
		var client httpcachev1betaconnect.HttpCacheServiceClient
		if tokenFile != "" {
			tokenSource, err := loadTokenFromFile(tokenFile)
			if err != nil {
				return fnerrors.Newf("failed to load token from file: %w", err)
			}

			client = fnapi.NewHttpCacheServiceClientWithToken(tokenSource)
		} else {
			cli, err := fnapi.NewHttpCacheServiceClient(ctx)
			if err != nil {
				return err
			}

			client = cli
		}

		req := connect.NewRequest(&httpcachev1beta.EnsureHttpCacheRequest{
			Name: name,
			Site: site,
		})

		resp, err := client.EnsureHttpCache(ctx, req)
		if err != nil {
			return fnerrors.Newf("failed to provision sccache cache: %w", err)
		}

		if resp.Msg.GetCacheEndpointUrl() == "" {
			return fnerrors.Newf("did not receive a valid cache endpoint")
		}

		var expiresAt *time.Time
		if resp.Msg.GetExpiresAt() != nil {
			t := resp.Msg.GetExpiresAt().AsTime()
			expiresAt = &t
		}

		// Parse the cache URL into base endpoint and key prefix.
		endpoint, keyPrefix, err := splitCacheURL(resp.Msg.GetCacheEndpointUrl())
		if err != nil {
			return fnerrors.Newf("failed to parse cache endpoint URL: %w", err)
		}

		token := resp.Msg.GetPassword()

		out := sccacheSetup{
			Endpoint:  endpoint,
			KeyPrefix: keyPrefix,
			Token:     token,
			ExpiresAt: expiresAt,
			Site:      resp.Msg.GetSite(),
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
			fmt.Fprintf(stdout, "SCCACHE_WEBDAV_ENDPOINT=%s\n", out.Endpoint)
			fmt.Fprintf(stdout, "SCCACHE_WEBDAV_KEY_PREFIX=%s\n", out.KeyPrefix)
			fmt.Fprintf(stdout, "SCCACHE_WEBDAV_TOKEN=%s\n", out.Token)
		}

		return nil
	})
}

func newCreateSccacheTokenCmd() *cobra.Command {
	var name string
	var expiresIn time.Duration
	var tokenFile, scope string

	return fncobra.Cmd(&cobra.Command{
		Use:   "create-token",
		Short: "Create a revokable token for accessing the sccache cache.",
	}).WithFlags(func(flags *pflag.FlagSet) {
		flags.StringVar(&name, "cache_name", "", "Select a cache to grant access to. By default, all caches can be accessed.")
		fncobra.DurationVar(flags, &expiresIn, "expires_in", 24*time.Hour, "Duration until the token expires (max 90 days).")
		flags.StringVar(&tokenFile, "token", "token.json", "Write token to this file in JSON format.")
		flags.StringVar(&scope, "scope", "user", "Set the scope of the generated access token. Valid options: `tenant`, `user`. Tokens with user scope are bound to the tenant membership of the current user.")
	}).Do(func(ctx context.Context) error {
		httpCacheClient, err := fnapi.NewHttpCacheServiceClient(ctx)
		if err != nil {
			return err
		}

		policyResp, err := httpCacheClient.GetAccessPolicy(ctx, connect.NewRequest(&httpcachev1beta.GetAccessPolicyRequest{
			Name: name,
		}))
		if err != nil {
			return fnerrors.Newf("failed to get access policy: %w", err)
		}

		requiredPerms := policyResp.Msg.GetRequiredPermission()
		if len(requiredPerms) == 0 {
			return fnerrors.New("no permissions required for this cache (unexpected)")
		}

		tokenSource, err := auth.LoadDefaults()
		if err != nil {
			return fnerrors.InvocationError("sccache", "failed to get authentication token: %w", err)
		}

		iamClient, err := iam.NewClient(ctx, tokenSource)
		if err != nil {
			return fnerrors.InvocationError("sccache", "failed to create IAM client: %w", err)
		}
		defer iamClient.Close()

		suffix := ids.NewRandomBase32ID(4)
		tokenName := fmt.Sprintf("sccache-%s-%s", name, suffix)
		expiresAt := time.Now().Add(expiresIn)

		req := &iamv1beta.CreateRevokableTokenRequest{
			Name:        tokenName,
			Description: fmt.Sprintf("sccache access token for cache %q", name),
			ExpiresAt:   timestamppb.New(expiresAt),
			Access: &iamv1beta.AccessPolicy{
				Grants: requiredPerms,
			},
		}

		switch scope {
		case "tenant":
			req.Scope = iamv1beta.RevokableToken_TENANT_SCOPE

		case "user":
			req.Scope = iamv1beta.RevokableToken_TENANT_MEMBERSHIP_SCOPE
		}

		resp, err := iamClient.Tokens.CreateRevokableToken(ctx, req)
		if err != nil {
			return fnerrors.InvocationError("token", "failed to create token: %w", err)
		}

		fmt.Fprintf(console.Stdout(ctx), "Token ID:    %s\n", resp.Token.GetTokenId())
		fmt.Fprintf(console.Stdout(ctx), "Name:        %s\n", resp.Token.GetName())
		fmt.Fprintf(console.Stdout(ctx), "Expires At:  %s\n", expiresAt.Format(time.RFC3339))

		if err := writeGradleTokenToFile(tokenFile, resp.BearerToken); err != nil {
			return fnerrors.InvocationError("token", "failed to write token to file: %w", err)
		}

		fmt.Fprintf(console.Stdout(ctx), "You can set up your sccache config with:\n")

		style := colors.Ctx(ctx)
		cmd := fmt.Sprintf("nsc cache sccache setup --token %s", tokenFile)
		if name != "" {
			cmd = fmt.Sprintf("%s --name %s", cmd, name)
		}

		fmt.Fprintf(console.Stdout(ctx), "  %s\n", style.Highlight.Apply(cmd))

		return nil
	})
}

// splitCacheURL parses a cache URL like "https://host:port/some/path/" into
// base endpoint ("https://host:port") and key prefix ("/some/path/").
func splitCacheURL(rawURL string) (string, string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", "", fmt.Errorf("invalid URL %q: %w", rawURL, err)
	}

	endpoint := fmt.Sprintf("%s://%s", u.Scheme, u.Host)
	keyPrefix := strings.TrimPrefix(u.Path, "/")

	return endpoint, keyPrefix, nil
}

type sccacheSetup struct {
	Endpoint  string     `json:"endpoint,omitempty"`
	KeyPrefix string     `json:"key_prefix,omitempty"`
	Token     string     `json:"token,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	Site      string     `json:"site,omitempty"`
}
