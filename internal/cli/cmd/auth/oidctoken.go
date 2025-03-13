// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package auth

import (
	"context"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/auth"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
)

func NewExchangeOIDCTokenCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "exchange-oidc-token",
		Short:  "Generate a Namespace Cloud token from a OIDC token.",
		Args:   cobra.NoArgs,
		Hidden: true,
	}

	token := cmd.Flags().String("token", "", "The OIDC token to use for authentication.")
	tenantId := cmd.Flags().String("tenant_id", "", "What tenant to authenticate.")

	return fncobra.Cmd(cmd).Do(func(ctx context.Context) error {
		if *token == "" {
			return fnerrors.Newf("--token is required")
		}

		if *tenantId == "" {
			return fnerrors.Newf("--tenant_id is required")
		}

		res, err := fnapi.ExchangeOIDCToken(ctx, *tenantId, *token)
		if err != nil {
			return err
		}

		if res.Tenant != nil {
			printLoginInfo(ctx, res.Tenant)
		}

		return auth.StoreTenantToken(res.TenantToken)
	})
}
