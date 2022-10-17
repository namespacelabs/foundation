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
	"namespacelabs.dev/foundation/internal/fnerrors/format"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/std/cfg"
)

func NewLintCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lint [path/to/package]...",
		Short: "Verify if package definitions are correct.",
	}

	var (
		env  cfg.Context
		locs fncobra.Locations
	)

	return fncobra.Cmd(cmd).
		With(
			fncobra.ParseEnv(&env),
			fncobra.ParseLocations(&locs, &env, fncobra.ParseLocationsOpts{ReturnAllIfNoneSpecified: true})).
		Do(func(ctx context.Context) error {
			for _, loc := range locs.Locs {
				fmt.Fprintln(console.Stderr(ctx), "Checking", loc.AsPackageName())
				if _, err := parsing.LoadPackageByName(ctx, env, loc.AsPackageName()); err != nil {
					fmt.Fprintln(console.Stderr(ctx))
					format.Format(console.Stderr(ctx), err, format.WithStyle(colors.WithColors))
					fmt.Fprintln(console.Stderr(ctx))
				}
			}
			return nil
		})
}
