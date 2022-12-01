// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package mod

import (
	"context"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/internal/parsing/module"
)

func newGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <module-uri>",
		Short: "Gets the latest version of the specified module.",
		Args:  cobra.ExactArgs(1),

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			root, err := module.FindRootWithArgs(ctx, ".", parsing.ModuleAtArgs{SkipAPIRequirements: true})
			if err != nil {
				return err
			}

			dep, err := parsing.ResolveModuleVersion(ctx, args[0])
			if err != nil {
				return err
			}

			if _, err := parsing.DownloadModule(ctx, dep, false); err != nil {
				return err
			}

			newData := root.EditableWorkspace().WithSetDependency(dep)
			if newData != nil {
				return rewriteWorkspace(ctx, root, newData)
			}

			return nil
		}),
	}

	return cmd
}
