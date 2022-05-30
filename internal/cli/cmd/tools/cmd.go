// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package tools

import (
	"github.com/spf13/cobra"
)

func NewToolsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "tools",
		Short:   "Tool-related commands.",
		Aliases: []string{"t", "tool"},
	}

	cmd.AddCommand(newGRPCurlCmd())
	cmd.AddCommand(newKubeCtlCmd())
	cmd.AddCommand(newOctantCmd())
	cmd.AddCommand(newGlooctlCmd())

	return cmd
}
