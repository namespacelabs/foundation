// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package auth

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
)

func NewCheckCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "check",
		Short:  "Returns information on whether the caller is still authenticated to Namespace.",
		Args:   cobra.NoArgs,
		Hidden: true,
	}

	return fncobra.Cmd(cmd).Do(func(ctx context.Context) error {
		t, err := fnapi.FetchToken(ctx)

		m := map[string]any{}
		if err == nil {
			claims, err := t.Claims(ctx)
			if err != nil {
				return err
			}

			m["session_token"] = t.IsSessionToken()
			m["expires_at"] = claims.ExpiresAt.Time.Format(time.RFC3339)
		} else {
			var x *fnerrors.ReauthErr

			if errors.As(err, &x) {
				m["invalid"] = x.Why
			} else {
				return err
			}
		}

		enc := json.NewEncoder(console.Stdout(ctx))
		enc.SetIndent("", "  ")
		return enc.Encode(m)
	})
}
