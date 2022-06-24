// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package create

import (
	"context"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/workspace"
)

func runGoInitCmdIfNeeded(ctx context.Context, root *workspace.Root, rootCmd *cobra.Command) error {
	f, err := root.FS().Open("go.mod")
	if err == nil {
		f.Close()
		return nil
	}

	return runGoInitCmd(ctx, root, rootCmd)
}

func runGoInitCmd(ctx context.Context, root *workspace.Root, rootCmd *cobra.Command) error {
	return runCommand(ctx, rootCmd, []string{"sdk", "go", "mod", "init", root.Workspace.ModuleName})
}
