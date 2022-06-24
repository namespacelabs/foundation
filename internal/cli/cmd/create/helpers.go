// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package create

import (
	"context"

	"github.com/spf13/cobra"
)

func runCommand(ctx context.Context, cmd *cobra.Command, args []string) error {
	cmdCopy := *cmd
	cmdCopy.SetArgs(args)
	cmdCopy.Flags().Parse(args)
	return cmdCopy.ExecuteContext(ctx)
}
