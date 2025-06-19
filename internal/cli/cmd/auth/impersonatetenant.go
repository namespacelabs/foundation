// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	localauth "namespacelabs.dev/foundation/internal/auth"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/integrations/api/iam"
	"namespacelabs.dev/integrations/auth"
	"namespacelabs.dev/integrations/auth/aws"
)

func NewImpersonateTenantCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "impersonate-tenant",
		Short:  "Signs in as a tenant, using a partner account.",
		Args:   cobra.NoArgs,
		Hidden: true,
	}

	identityPool := cmd.Flags().String("aws_identity_pool", "", "UUID of the identity pool.")
	tenantId := cmd.Flags().String("tenant_id", "", "What tenant to authenticate.")
	partnerId := cmd.Flags().String("partner_id", "", "What partner owns the tenant.")
	duration := cmd.Flags().Duration("duration", time.Hour, "How long will the impersonation token last.")

	return fncobra.Cmd(cmd).Do(func(ctx context.Context) error {
		if *identityPool == "" {
			return fnerrors.Newf("aws_identity_pool is required")
		}

		if *tenantId == "" {
			return fnerrors.Newf("tenant_id is required")
		}

		if *partnerId == "" {
			return fnerrors.Newf("partner_id is required")
		}

		tokenSource, err := aws.Federation(ctx, *identityPool, *partnerId)
		if err != nil {
			return err
		}

		iam, err := iam.NewClient(ctx, tokenSource)
		if err != nil {
			return err
		}

		token, err := auth.TenantTokenSource(iam, *tenantId).IssueToken(ctx, *duration, false)
		if err != nil {
			return err
		}

		stdout := console.Stdout(ctx)

		fmt.Fprintf(stdout, "\nYou are now impersonating %q, for %v.\n", *tenantId, *duration)

		return localauth.StoreTenantToken(token)
	})
}
