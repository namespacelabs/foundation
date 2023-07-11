// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package auth

import "github.com/spf13/cobra"

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

	return cmd
}
