// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package secrets

import (
	"context"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/fnerrors"
)

func newDeleteCmd() *cobra.Command {
	var secretKey string
	var rawtext bool
	var locs fncobra.Locations

	return fncobra.
		Cmd(&cobra.Command{
			Use:   "delete",
			Short: "Deletes the specified secret value.",
			Args:  cobra.MaximumNArgs(1),
		}).
		WithFlags(func(cmd *cobra.Command) {
			cmd.Flags().StringVar(&secretKey, "secret", "", "The secret key, in {package_name}:{name} format.")
			cmd.Flags().BoolVar(&rawtext, "rawtext", rawtext, "If set to true, the bundle is not encrypted (use for testing purposes only).")
			_ = cmd.MarkFlagRequired("secret")
		}).
		With(fncobra.ParseLocations(&locs, &fncobra.ParseLocationsOpts{RequireSingle: true})).
		Do(func(ctx context.Context) error {
			loc, bundle, err := loadBundleFromArgs(ctx, locs.All[0], nil)
			if err != nil {
				return err
			}

			key, err := parseKey(secretKey)
			if err != nil {
				return err
			}

			if !bundle.Delete(key.PackageName, key.Key) {
				return fnerrors.New("no such key")
			}

			return writeBundle(ctx, loc, bundle, !rawtext)
		})
}
