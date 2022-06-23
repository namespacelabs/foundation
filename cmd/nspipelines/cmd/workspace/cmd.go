// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package workspace

import "github.com/spf13/cobra"

func NewWorkspaceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "workspace",
	}

	cmd.AddCommand(newInfoCmd())
	cmd.AddCommand(newPrepareCmd())

	return cmd
}
