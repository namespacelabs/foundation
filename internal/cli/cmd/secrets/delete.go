// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package secrets

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/provision"
)

func newDeleteCmd() *cobra.Command {
	var (
		secretKey string
		rawtext   bool
		locs      fncobra.Locations
		env       provision.Env
	)

	return fncobra.
		Cmd(&cobra.Command{
			Use:   "delete {path/to/server} --secret {package_name}:{secret_name}",
			Short: "Deletes the specified secret value.",
			Args:  cobra.MaximumNArgs(1),
		}).
		WithFlags(func(flags *pflag.FlagSet) {
			flags.StringVar(&secretKey, "secret", "", "The secret key, in {package_name}:{name} format.")
			flags.BoolVar(&rawtext, "rawtext", rawtext, "If set to true, the bundle is not encrypted (use for testing purposes only).")
			_ = cobra.MarkFlagRequired(flags, "secret")
		}).
		With(
			fncobra.FixedEnv(&env, "dev"),
			fncobra.ParseLocations(&locs, &fncobra.ParseLocationsOpts{RequireSingle: true})).
		Do(func(ctx context.Context) error {
			loc, bundle, err := loadBundleFromArgs(ctx, env, locs.Locs[0], nil)
			if err != nil {
				return err
			}

			key, err := parseKey(secretKey, string(loc.loc.PackageName))
			if err != nil {
				return err
			}

			if !bundle.Delete(key.PackageName, key.Key) {
				return fnerrors.New("no such key")
			}

			return writeBundle(ctx, loc, bundle, !rawtext)
		})
}
