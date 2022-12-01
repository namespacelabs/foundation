// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package mod

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/internal/parsing/module"
)

func newDownloadCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "download",
		Short: "Downloads all referenced modules.",

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			root, err := module.FindRootWithArgs(ctx, ".", parsing.ModuleAtArgs{SkipAPIRequirements: true})
			if err != nil {
				return err
			}

			for _, dep := range root.Workspace().Proto().Dep {
				mod, err := parsing.DownloadModule(ctx, dep, force)
				if err != nil {
					return err
				}

				fmt.Fprintf(console.Stdout(ctx), "Downloaded %s: %s\n", mod.ModuleName, mod.Version)
			}

			return nil
		}),
	}

	cmd.Flags().BoolVar(&force, "force", force, "Download a module even if it already exists locally.")

	return cmd
}
