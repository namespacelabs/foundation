// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/module"
)

func NewModCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mod",
		Short: "Module related operations (e.g. download, get, tidy).",
	}

	cmd.AddCommand(NewTidyCmd())
	cmd.AddCommand(newModDownloadCmd())
	cmd.AddCommand(newModGetCmd())

	return cmd
}

func newModDownloadCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "download",
		Short: "Downloads all referenced modules.",

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			root, err := module.FindRootWithArgs(ctx, ".", workspace.ModuleAtArgs{SkipAPIRequirements: true})
			if err != nil {
				return err
			}

			for _, dep := range root.Workspace().Dep {
				mod, err := workspace.DownloadModule(ctx, dep, force)
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

func newModGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <module-uri>",
		Short: "Gets the latest version of the specified module.",
		Args:  cobra.ExactArgs(1),

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			root, err := module.FindRootWithArgs(ctx, ".", workspace.ModuleAtArgs{SkipAPIRequirements: true})
			if err != nil {
				return err
			}

			dep, err := workspace.ResolveModuleVersion(ctx, args[0])
			if err != nil {
				return err
			}

			if _, err := workspace.DownloadModule(ctx, dep, false); err != nil {
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
