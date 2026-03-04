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
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
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

	cmd := fncobra.Cmd(&cobra.Command{
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

	_ = cmd.MarkFlagRequired("cache_name")
	return cmd
}

func newCreateSccacheTokenCmd() *cobra.Command {
	return newCreateCacheTokenCmd(createTokenConfig{
		Short:       "Create a revokable token for accessing the sccache cache.",
		CacheLabel:  "sccache",
		TokenPrefix: "sccache",
		SetupCmd:    "nsc cache sccache setup",
		GetRequiredPerms: func(ctx context.Context, cacheName string) ([]*iamv1beta.Permission, error) {
			httpCacheClient, err := fnapi.NewHttpCacheServiceClient(ctx)
			if err != nil {
				return nil, err
			}

			policyResp, err := httpCacheClient.GetAccessPolicy(ctx, connect.NewRequest(&httpcachev1beta.GetAccessPolicyRequest{
				Name: cacheName,
			}))
			if err != nil {
				return nil, fnerrors.Newf("failed to get access policy: %w", err)
			}

			return policyResp.Msg.GetRequiredPermission(), nil
		},
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
