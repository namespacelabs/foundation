// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package auth

import (
	"context"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/auth"
	"namespacelabs.dev/foundation/internal/circleci"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
)

func NewExchangeCircleCITokenCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "exchange-circleci-token",
		Short:  "Generate a Namespace Cloud token from a CircleCI JWT.",
		Args:   cobra.NoArgs,
		Hidden: true,
	}

	return fncobra.Cmd(cmd).Do(func(ctx context.Context) error {
		if !circleci.IsRunningInCircleci() {
			return fnerrors.Newf("not running in CircleCI")
		}

		token, err := circleci.GetOidcTokenV2()
		if err != nil {
			return err
		}

		res, err := fnapi.ExchangeCircleciToken(ctx, token)
		if err != nil {
			return err
		}

		if res.Tenant != nil {
			printLoginInfo(ctx, res.Tenant)
		}

		return auth.StoreTenantToken(res.TenantToken)
	})
}
