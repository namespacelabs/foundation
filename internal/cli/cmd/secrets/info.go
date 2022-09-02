// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package secrets

import (
	"context"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/provision"
)

func newInfoCmd() *cobra.Command {
	var (
		locs fncobra.Locations
		env  provision.Env
	)

	return fncobra.Cmd(
		&cobra.Command{
			Use:   "info {path/to/server}",
			Short: "Describes the contents of the specified server's secrets archive.",
			Args:  cobra.MaximumNArgs(1),
		}).
		With(
			fncobra.FixedEnv(&env, "dev"),
			fncobra.ParseLocations(&locs, &fncobra.ParseLocationsOpts{RequireSingle: true})).
		Do(func(ctx context.Context) error {
			_, bundle, err := loadBundleFromArgs(ctx, env, locs.Locs[0], nil)
			if err != nil {
				return err
			}

			bundle.DescribeTo(console.Stdout(ctx))
			return nil
		})
}
