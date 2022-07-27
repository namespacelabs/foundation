// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package secrets

import (
	"context"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
)

func newInfoCmd() *cobra.Command {
	var locs fncobra.Locations
	return fncobra.CmdWithHandler(
		&cobra.Command{
			Use:   "info",
			Short: "Describes the contents of the specified server's secrets archive.",
			Args:  cobra.MaximumNArgs(1),
		},
		func(ctx context.Context, args []string) error {
			_, bundle, err := loadBundleFromArgs(ctx, &locs, nil)
			if err != nil {
				return err
			}

			bundle.DescribeTo(console.Stdout(ctx))
			return nil
		},
		fncobra.NewLocationsParser(&locs),
	)
}
