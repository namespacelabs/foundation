// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/fnapi"
)

func NewCheckLoginCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "check-login",
		Short:  "Return a successful exit code if the caller is still authenticated to Namespace.",
		Args:   cobra.NoArgs,
		Hidden: true,
	}

	var dur time.Duration
	cmd.Flags().DurationVar(&dur, "duration", time.Minute*5, "Fail if the current session does not last at least this much more time.")

	return fncobra.Cmd(cmd).Do(func(ctx context.Context) error {
		tok, err := fnapi.FetchToken(ctx)
		if err != nil {
			return err
		}

		claims, err := tok.Claims(ctx)
		if err != nil {
			return err
		}

		// Do an expiry check for non-session tokens only.
		if expires := time.Until(claims.ExpiresAt.Time); expires < dur && !tok.IsSessionToken() {
			if expires < 0 {
				return fmt.Errorf("token expired %s", humanize.Time(claims.ExpiresAt.Time))
			}

			return fmt.Errorf("token expires %s", humanize.Time(claims.ExpiresAt.Time))
		}

		// For session tokens we need to try issuing a tenant token
		// regardless of expiry, because the session could have been revoked.
		_, err = tok.IssueToken(ctx, dur, fnapi.IssueTenantTokenFromSession, true)
		return err
	})
}
