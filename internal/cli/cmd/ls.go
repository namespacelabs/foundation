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
	"namespacelabs.dev/foundation/internal/planning"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/module"
)

func NewLsCmd() *cobra.Command {
	var (
		env planning.Context
	)

	return fncobra.Cmd(
		&cobra.Command{
			Use:     "ls",
			Short:   "List all known packages in the current workspace.",
			Args:    cobra.NoArgs,
			Aliases: []string{"list"},
		}).
		With(fncobra.FixedEnv(&env, "dev")).
		Do(func(ctx context.Context) error {
			root, err := module.FindRoot(ctx, ".")
			if err != nil {
				return err
			}

			list, err := workspace.ListSchemas(ctx, env, root)
			if err != nil {
				return err
			}

			stdout := console.Stdout(ctx)
			for _, s := range list.Locations {
				fmt.Fprintln(stdout, s.Rel())
			}

			return nil
		})
}
