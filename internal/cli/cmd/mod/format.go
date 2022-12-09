// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package mod

import (
	"context"
	"strings"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/internal/parsing/module"
)

func newFormatCmd() *cobra.Command {
	return fncobra.Cmd(
		&cobra.Command{
			Use:     "format",
			Aliases: []string{"fmt"},
			Short:   "Format the workspace file.",
			Args:    cobra.NoArgs,
		}).
		Do(func(ctx context.Context) error {
			root, err := module.FindRootWithArgs(ctx, ".", parsing.ModuleAtArgs{SkipModuleNameValidation: true})
			if err != nil {
				return err
			}

			return rewriteWorkspace(ctx, root, root.EditableWorkspace().WithModuleName(strings.ToLower(root.ModuleName())))
		})
}
