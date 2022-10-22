// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package create

import (
	"context"

	"namespacelabs.dev/foundation/internal/parsing"
)

func runGoInitCmdIfNeeded(ctx context.Context, root *parsing.Root, runCommand func(ctx context.Context, args []string) error) error {
	f, err := root.ReadWriteFS().Open("go.mod")
	if err == nil {
		f.Close()
		return nil
	}

	return runCommand(ctx, []string{"sdk", "go", "mod", "init", root.Workspace().ModuleName()})
}
