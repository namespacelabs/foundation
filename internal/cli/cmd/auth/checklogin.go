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
		Use:   "check-login",
		Short: "Return a successful exit code if the caller is still authenticated to Namespace.",
		Args:  cobra.NoArgs,
	}

	dur := fncobra.Duration(cmd.Flags(), "duration", 5*time.Minute, "Fail if the current session does not last at least this much more time.")

	return fncobra.Cmd(cmd).Do(func(ctx context.Context) error {
		tok, err := fnapi.FetchToken(ctx)
		if err != nil {
			return err
		}

		expiry, ok, err := tok.ExpiresAt(ctx)
		if err != nil {
			return err
		}

		// ok is false for revokable tokens which are validated server-side.
		if ok {
			// Do an expiry check for non-session tokens only.
			if expires := time.Until(expiry); expires < *dur && !tok.IsSessionToken() {
				if expires < 0 {
					return fmt.Errorf("token expired %s", humanize.Time(expiry))
				}

				return fmt.Errorf("token expires %s", humanize.Time(expiry))
			}
		}

		// For session tokens and revokable tokens we need to try issuing a tenant token
		// regardless of expiry, because the session could have been revoked.
		_, err = tok.IssueToken(ctx, *dur, true)
		return err
	})
}
