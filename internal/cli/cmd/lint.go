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
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/planning"
	"namespacelabs.dev/foundation/workspace"
)

func NewLintCmd() *cobra.Command {
	var (
		env  planning.Context
		locs fncobra.Locations
	)

	return fncobra.
		Cmd(&cobra.Command{
			Use:   "lint [path/to/package]...",
			Short: "Verify if package definitions are correct.",
		}).
		With(
			fncobra.ParseEnv(&env),
			fncobra.ParseLocations(&locs, &env, &fncobra.ParseLocationsOpts{DefaultToAllWhenEmpty: true})).
		Do(func(ctx context.Context) error {
			for _, loc := range locs.Locs {
				fmt.Fprintln(console.Stderr(ctx), "Checking", loc.AsPackageName())
				if _, err := workspace.LoadPackageByName(ctx, env, loc.AsPackageName()); err != nil {
					fmt.Fprintln(console.Stderr(ctx))
					fnerrors.Format(console.Stderr(ctx), err, fnerrors.WithStyle(colors.WithColors))
					fmt.Fprintln(console.Stderr(ctx))
				}
			}
			return nil
		})
}
