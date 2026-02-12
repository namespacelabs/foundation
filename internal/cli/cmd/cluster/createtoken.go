// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	iamv1beta "buf.build/gen/go/namespace/cloud/protocolbuffers/go/proto/namespace/cloud/iam/v1beta"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"google.golang.org/protobuf/types/known/timestamppb"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/go-ids"
	"namespacelabs.dev/integrations/api/iam"
	"namespacelabs.dev/integrations/auth"
)

type createTokenConfig struct {
	Short       string
	CacheLabel  string // e.g. "Gradle" or "sccache", used in help text and descriptions.
	TokenPrefix string // e.g. "gradle-cache" or "sccache", used as token name prefix.
	SetupCmd    string // e.g. "nsc gradle cache setup" or "nsc cache sccache setup".

	GetRequiredPerms func(ctx context.Context, cacheName string) ([]*iamv1beta.Permission, error)
}

func newCreateCacheTokenCmd(cfg createTokenConfig) *cobra.Command {
	var name string
	var expiresIn time.Duration
	var tokenFile, scope string

	return fncobra.Cmd(&cobra.Command{
		Use:   "create-token",
		Short: cfg.Short,
	}).WithFlags(func(flags *pflag.FlagSet) {
		flags.StringVar(&name, "cache_name", "", fmt.Sprintf("Select a %s cache to grant access to. By default, all %s caches can be accessed.", cfg.CacheLabel, cfg.CacheLabel))
		fncobra.DurationVar(flags, &expiresIn, "expires_in", 90*24*time.Hour, "Duration until the token expires.")
		flags.StringVar(&tokenFile, "token", "token.json", "Write token to this file in JSON format.")
		flags.StringVar(&scope, "scope", "user", "Set the scope of the generated access token. Valid options: `tenant`, `user`. Tokens with user scope are bound to the tenant membership of the current user.")
	}).Do(func(ctx context.Context) error {
		requiredPerms, err := cfg.GetRequiredPerms(ctx, name)
		if err != nil {
			return err
		}

		if len(requiredPerms) == 0 {
			return fnerrors.New("no permissions required for this cache (unexpected)")
		}

		tokenSource, err := auth.LoadDefaults()
		if err != nil {
			return fnerrors.InvocationError(cfg.TokenPrefix, "failed to get authentication token: %w", err)
		}

		iamClient, err := iam.NewClient(ctx, tokenSource)
		if err != nil {
			return fnerrors.InvocationError(cfg.TokenPrefix, "failed to create IAM client: %w", err)
		}
		defer iamClient.Close()

		suffix := ids.NewRandomBase32ID(4)
		tokenName := fmt.Sprintf("%s-%s-%s", cfg.TokenPrefix, name, suffix)
		expiresAt := time.Now().Add(expiresIn)

		req := &iamv1beta.CreateRevokableTokenRequest{
			Name:        tokenName,
			Description: fmt.Sprintf("%s access token for cache %q", cfg.CacheLabel, name),
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

		if err := writeTokenToFile(tokenFile, resp.BearerToken); err != nil {
			return fnerrors.InvocationError("token", "failed to write token to file: %w", err)
		}

		fmt.Fprintf(console.Stdout(ctx), "\nWrote token contents to %q\n\n", tokenFile)
		fmt.Fprintf(console.Stdout(ctx), "You can set up your %s config with:\n", cfg.CacheLabel)

		style := colors.Ctx(ctx)
		cmd := fmt.Sprintf("%s --token %s", cfg.SetupCmd, tokenFile)
		if name != "" {
			cmd = fmt.Sprintf("%s --cache_name %s", cmd, name)
		}

		fmt.Fprintf(console.Stdout(ctx), "  %s\n", style.Highlight.Apply(cmd))

		return nil
	})
}

func writeTokenToFile(path string, bearerToken string) error {
	tokenData := map[string]string{
		"bearer_token": bearerToken,
	}

	bb, err := json.MarshalIndent(tokenData, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, bb, 0600)
}
