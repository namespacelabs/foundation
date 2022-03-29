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

func NewLintCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lint",
		Short: "Verify if package definitions are correct.",
		Args:  cobra.RangeArgs(0, 1),

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			if len(args) == 0 {
				root, err := module.FindRoot(ctx, ".")
				if err != nil {
					return err
				}

				list, err := workspace.ListSchemas(ctx, root)
				if err != nil {
					return err
				}

				for _, loc := range list.Locations {
					fmt.Fprintln(console.Stderr(ctx), "Checking", loc.AsPackageName())
					if _, err := workspace.LoadPackage(ctx, root, loc); err != nil {
						fmt.Fprintln(console.Stderr(ctx), loc.AsPackageName(), err)
					}
				}

				return nil
			}

			root, loc, err := module.PackageAtArgs(ctx, args)
			if err != nil {
				return err
			}

			_, err = workspace.LoadPackage(ctx, root, loc)
			return err
		}),
	}

	return cmd
}