// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package create

import (
	"context"

	"namespacelabs.dev/foundation/workspace"
)

func runGoInitCmdIfNeeded(ctx context.Context, root *workspace.Root, runCommand func(ctx context.Context, args []string) error) error {
	f, err := root.FS().Open("go.mod")
	if err == nil {
		f.Close()
		return nil
	}

	return runCommand(ctx, []string{"sdk", "go", "mod", "init", root.Workspace().ModuleName})
}
