// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package token

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	v1beta "buf.build/gen/go/namespace/cloud/protocolbuffers/go/proto/namespace/cloud/iam/v1beta"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/tui"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/integrations/api/iam"
	"namespacelabs.dev/integrations/auth"
)

func NewTokenCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "token",
		Short: "Manage revokable tokens.",
	}

	cmd.AddCommand(NewListCmd())
	cmd.AddCommand(NewCreateCmd())
	cmd.AddCommand(NewRevokeCmd())

	return cmd
}

func NewListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List revokable tokens.",
		Args:  cobra.NoArgs,
	}

	output := cmd.Flags().StringP("output", "o", "table", "Output format: table, json")
	includeRevoked := cmd.Flags().Bool("include_revoked", false, "Include revoked tokens in the list")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		tokenSource, err := auth.LoadDefaults()
		if err != nil {
			return fnerrors.InvocationError("token", "failed to get authentication token: %w", err)
		}

		client, err := iam.NewClient(ctx, tokenSource)
		if err != nil {
			return fnerrors.InvocationError("token", "failed to create iam client: %w", err)
		}
		defer client.Close()

		req := &v1beta.ListRevokableTokensRequest{
			IncludeRevoked: *includeRevoked,
		}

		resp, err := client.Tokens.ListRevokableTokens(ctx, req)
		if err != nil {
			return fnerrors.InvocationError("token", "failed to list revokable tokens: %w", err)
		}

		switch *output {
		case "json":
			var b bytes.Buffer
			fmt.Fprint(&b, "[")
			for k, token := range resp.Tokens {
				if k > 0 {
					fmt.Fprint(&b, ",")
				}

				bb, err := protojson.MarshalOptions{UseProtoNames: true, Multiline: true}.Marshal(token)
				if err != nil {
					return err
				}

				fmt.Fprintf(&b, "\n%s", bb)
			}
			fmt.Fprint(&b, "\n]\n")

			console.Stdout(ctx).Write(b.Bytes())

			return nil
		case "table":
			return printTokensTable(ctx, resp.Tokens)
		default:
			return fnerrors.BadInputError("invalid output format: %s", *output)
		}
	})

	return cmd
}

func NewCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a revokable token.",
		Args:  cobra.NoArgs,
	}

	name := cmd.Flags().String("name", "", "A unique name for the token within the tenant")
	description := cmd.Flags().StringP("description", "d", "", "A human-readable description of the token's purpose")
	expiresIn := fncobra.Duration(cmd.Flags(), "expires_in", 24*time.Hour, "Duration until the token expires (max 90 days)")
	grants := cmd.Flags().StringArray("grant", nil, "Grant permission as JSON object (can be specified multiple times). Format: {\"resource_type\":\"...\",\"resource_id\":\"...\",\"actions\":[\"...\"]}")
	output := cmd.Flags().StringP("output", "o", "table", "Output format: table, json, token")
	tokenFile := cmd.Flags().String("token_file", "", "Write token to this file in JSON format")
	userScope := cmd.Flags().Bool("user", false, "Create a token bound to the current user's workspace membership.")

	cmd.MarkFlagRequired("name")
	cmd.MarkFlagRequired("grant")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {

		tokenSource, err := auth.LoadDefaults()
		if err != nil {
			return fnerrors.InvocationError("token", "failed to get authentication token: %w", err)
		}

		client, err := iam.NewClient(ctx, tokenSource)
		if err != nil {
			return fnerrors.InvocationError("token", "failed to create iam client: %w", err)
		}
		defer client.Close()

		permissions, err := parseGrants(*grants)
		if err != nil {
			return fnerrors.BadInputError("failed to parse grants: %w", err)
		}

		expiresAt := time.Now().Add(*expiresIn)
		req := &v1beta.CreateRevokableTokenRequest{
			Name:        *name,
			Description: *description,
			ExpiresAt:   timestamppb.New(expiresAt),
			Access: &v1beta.AccessPolicy{
				Grants: permissions,
			},
		}

		if *userScope {
			req.Scope = v1beta.RevokableToken_TENANT_MEMBERSHIP_SCOPE
		}

		resp, err := client.Tokens.CreateRevokableToken(ctx, req)
		if err != nil {
			return fnerrors.InvocationError("token", "failed to create revokable token: %w", err)
		}

		if *tokenFile != "" {
			if err := writeTokenToFile(*tokenFile, resp.BearerToken); err != nil {
				return fnerrors.InvocationError("token", "failed to write token to file: %w", err)
			}
			fmt.Fprintf(console.Stdout(ctx), "Token written to %s\n", *tokenFile)
		}

		switch *output {
		case "json":
			bb, err := protojson.MarshalOptions{UseProtoNames: true, Multiline: true}.Marshal(resp)
			if err != nil {
				return err
			}

			fmt.Fprintln(console.Stdout(ctx), string(bb))
			return nil

		case "token":
			return printTokenJSON(ctx, resp.BearerToken)

		case "table":
			return printTokenCreated(ctx, resp, *tokenFile == "")

		default:
			return fnerrors.BadInputError("invalid output format: %s", *output)
		}
	})

	return cmd
}

func NewRevokeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "revoke",
		Short: "Revoke a token.",
		Args:  cobra.NoArgs,
	}

	tokenId := cmd.Flags().String("token_id", "", "The token ID to revoke")

	cmd.MarkFlagRequired("token_id")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {

		tokenSource, err := auth.LoadDefaults()
		if err != nil {
			return fnerrors.InvocationError("token", "failed to get authentication token: %w", err)
		}

		client, err := iam.NewClient(ctx, tokenSource)
		if err != nil {
			return fnerrors.InvocationError("token", "failed to create iam client: %w", err)
		}
		defer client.Close()

		req := &v1beta.RevokeRevokableTokenRequest{
			TokenId: *tokenId,
		}

		_, err = client.Tokens.RevokeRevokableToken(ctx, req)
		if err != nil {
			return fnerrors.InvocationError("token", "failed to revoke token: %w", err)
		}

		fmt.Fprintf(console.Stdout(ctx), "Successfully revoked token %s\n", *tokenId)
		return nil
	})

	return cmd
}

func printTokensTable(ctx context.Context, tokens []*v1beta.RevokableToken) error {
	if len(tokens) == 0 {
		fmt.Fprintf(console.Stdout(ctx), "No tokens found.\n")
		return nil
	}

	cols := []tui.Column{
		{Key: "token_id", Title: "TOKEN ID", MinWidth: 20, MaxWidth: 50},
		{Key: "name", Title: "NAME", MinWidth: 10, MaxWidth: 30},
		{Key: "description", Title: "DESCRIPTION", MinWidth: 10, MaxWidth: 50},
		{Key: "created", Title: "CREATED", MinWidth: 20, MaxWidth: 20},
		{Key: "status", Title: "STATUS", MinWidth: 10, MaxWidth: 10},
	}

	rows := []tui.Row{}
	for _, token := range tokens {
		createdAt := ""
		if token.CreatedAt != nil {
			createdAt = token.CreatedAt.AsTime().Format(time.RFC3339)
		}

		status := "Active"
		if token.RevokedAt != nil {
			status = "Revoked"
		} else if token.ExpiresAt != nil && token.ExpiresAt.AsTime().Before(time.Now()) {
			status = "Expired"
		}

		row := tui.Row{
			"token_id":    token.TokenId,
			"name":        token.Name,
			"description": token.Description,
			"created":     createdAt,
			"status":      status,
		}
		rows = append(rows, row)
	}

	return tui.StaticTable(ctx, cols, rows)
}

func printTokenCreated(ctx context.Context, resp *v1beta.CreateRevokableTokenResponse, showToken bool) error {
	if resp.Token != nil {
		fmt.Fprintf(console.Stdout(ctx), "Token ID:     %s\n", resp.Token.TokenId)
		fmt.Fprintf(console.Stdout(ctx), "Name:         %s\n", resp.Token.Name)
		fmt.Fprintf(console.Stdout(ctx), "Description:  %s\n", resp.Token.Description)

		if resp.Token.CreatedAt != nil {
			fmt.Fprintf(console.Stdout(ctx), "Created At:   %s\n", resp.Token.CreatedAt.AsTime().Format(time.RFC3339))
		}

		if resp.Token.ExpiresAt != nil {
			fmt.Fprintf(console.Stdout(ctx), "Expires At:   %s\n", resp.Token.ExpiresAt.AsTime().Format(time.RFC3339))
		}
	}

	if showToken {
		fmt.Fprintf(console.Stdout(ctx), "\nBearer Token: %s\n", resp.BearerToken)
		fmt.Fprintf(console.Stdout(ctx), "\n⚠️  Save this token securely - it will not be shown again.\n")
	}

	return nil
}

func printTokenJSON(ctx context.Context, bearerToken string) error {
	tokenData := map[string]string{
		"bearer_token": bearerToken,
	}

	bb, err := json.MarshalIndent(tokenData, "", "  ")
	if err != nil {
		return err
	}

	fmt.Fprintln(console.Stdout(ctx), string(bb))
	return nil
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

func parseGrants(grants []string) ([]*v1beta.Permission, error) {
	var permissions []*v1beta.Permission

	for _, grant := range grants {
		var perm v1beta.Permission
		if err := protojson.Unmarshal([]byte(grant), &perm); err != nil {
			return nil, fnerrors.BadInputError("failed to parse grant JSON %q: %w", grant, err)
		}

		if perm.ResourceType == "" {
			return nil, fnerrors.BadInputError("grant %q: resource_type is required", grant)
		}

		if len(perm.Actions) == 0 {
			return nil, fnerrors.BadInputError("grant %q: at least one action is required", grant)
		}

		permissions = append(permissions, &perm)
	}

	return permissions, nil
}
