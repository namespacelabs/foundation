// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package github

import "github.com/spf13/cobra"

func NewGithubCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "github",
	}

	cmd.AddCommand(newAccessTokenCmd())
	cmd.AddCommand(newUpdateStatusCmd())
	cmd.AddCommand(newPullRequestCmd())

	return cmd
}
