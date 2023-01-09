// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package mod

import (
	"context"
	"fmt"
	"io"
	"path/filepath"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/findroot"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/frontend/cuefrontend"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
)

func newInitCmd(runCommand func(context.Context, []string) error) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init [module-path]",
		Short: "Initialize the module workspace with default values.",
		Args:  cobra.MinimumNArgs(1),

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			dir, err := filepath.Abs(".")
			if err != nil {
				return err
			}

			if findroot.LookForFile(cuefrontend.WorkspaceFile, cuefrontend.LegacyWorkspaceFile)(dir) {
				return fnerrors.New("workspace file aready exists.")
			}

			moduleName := args[0]
			fmt.Println("Creating initial workspace.")
			w := &schema.Workspace{
				ModuleName: moduleName,
				EnvSpec:    cfg.DefaultWorkspaceEnvironments,
			}

			mod, err := parsing.NewModule(ctx, dir, w)
			if err != nil {
				return err
			}

			if err = fnfs.WriteWorkspaceFile(ctx, nil, fnfs.ReadWriteLocalFS(dir), mod.DefinitionFile(), func(w io.Writer) error {
				return mod.FormatTo(w)
			}); err != nil {
				return err
			}

			fmt.Println("Running 'ns mod tidy' command to finalize the workspace.")
			return runCommand(ctx, []string{"mod", "tidy"})
		}),
	}
	return cmd
}
