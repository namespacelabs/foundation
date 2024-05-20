// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package auth

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnapi"
)

func NewAuthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Authentication related operations (e.g. login).",
	}

	cmd.AddCommand(NewLoginCmd())
	cmd.AddCommand(NewExchangeGithubTokenCmd())
	cmd.AddCommand(NewExchangeCircleCITokenCmd())
	cmd.AddCommand(newExchangeAwsCognitoCmd())
	cmd.AddCommand(newTrustAwsCognitoCmd())
	cmd.AddCommand(NewIssueIdTokenCmd())
	cmd.AddCommand(NewExchangeOIDCTokenCmd())
	cmd.AddCommand(NewGenerateDevTokenCmd())
	cmd.AddCommand(NewGenerateTokenCmd())
	cmd.AddCommand(NewCheckCmd())

	return cmd
}

func printLoginInfo(ctx context.Context, tenant *fnapi.Tenant) {
	if tenant.Name != "" {
		fmt.Fprintf(console.Stdout(ctx), "You are now logged into workspace %q, have a nice day.\n", tenant.Name)
	}
	if tenant.AppUrl != "" {
		fmt.Fprintf(console.Stdout(ctx), "You can inspect your instances at %s\n", tenant.AppUrl)
	}
}
