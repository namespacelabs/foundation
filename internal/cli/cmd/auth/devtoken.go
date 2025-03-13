// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package auth

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/auth"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
)

func NewGenerateDevTokenCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "generate-dev-token",
		Short:  "Generate a Namespace Cloud token for development purposes.",
		Args:   cobra.NoArgs,
		Hidden: true,
	}

	outputPath := cmd.Flags().String("output_to", "", "If specified, write the access token to this path.")

	return fncobra.Cmd(cmd).Do(func(ctx context.Context) error {
		res, err := fnapi.IssueDevelopmentToken(ctx)
		if err != nil {
			return err
		}

		if *outputPath != "" {
			if err := os.WriteFile(*outputPath, []byte(res), 0644); err != nil {
				return fnerrors.Newf("failed to write %q: %w", *outputPath, err)
			}
		} else {
			fmt.Fprintln(console.Stdout(ctx), res)
		}

		return nil
	})
}

func NewGenerateTokenCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "generate-token",
		Short:  "Generate a Namespace Cloud token.",
		Args:   cobra.NoArgs,
		Hidden: true,
	}

	outputPath := cmd.Flags().String("output_to", "", "If specified, write the access token to this path.")
	duration := cmd.Flags().Duration("duration", time.Minute, "How long the token should last. Default is 1 minute.")

	return fncobra.Cmd(cmd).Do(func(ctx context.Context) error {
		tok, err := fnapi.FetchToken(ctx)
		if err != nil {
			return err
		}

		token := ""

		// If non-session token
		if nakedToken, ok := tok.(*auth.Token); ok {
			token = nakedToken.BearerToken
		}

		if tok.IsSessionToken() {
			// Overwrite if it's session token
			token, err = tok.IssueToken(ctx, *duration, fnapi.IssueTenantTokenFromSession, false)
			if err != nil {
				return err
			}
		}

		if *outputPath != "" {
			if err := os.WriteFile(*outputPath, []byte(token), 0644); err != nil {
				return fnerrors.Newf("failed to write %q: %w", *outputPath, err)
			}
		} else {
			fmt.Fprintln(console.Stdout(ctx), token)
		}

		return nil
	})
}
