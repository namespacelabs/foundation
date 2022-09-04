// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package secrets

import (
	"context"
	"fmt"
	"unicode/utf8"

	"github.com/kr/text"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/secrets"
	"namespacelabs.dev/foundation/std/planning"
)

func newRevealCmd() *cobra.Command {
	var (
		locLoadingEnv          planning.Context
		secretKey, specificEnv string
		locs                   fncobra.Locations
	)

	return fncobra.
		Cmd(&cobra.Command{
			Use:   "reveal {path/to/server} --secret {package_name}:{name}",
			Short: "Reveals the specified secret value.",
			Args:  cobra.MaximumNArgs(1),
		}).
		WithFlags(func(flags *pflag.FlagSet) {
			flags.StringVar(&secretKey, "secret", "", "The secret key, in {package_name}:{name} format.")
			flags.StringVar(&specificEnv, "env", "", "If set, matches specified secret with the named environment (e.g. dev, or prod).")
			_ = cobra.MarkFlagRequired(flags, "secret")
		}).
		With(
			fncobra.FixedEnv(&locLoadingEnv, "dev"),
			fncobra.ParseLocations(&locs, &locLoadingEnv, &fncobra.ParseLocationsOpts{RequireSingle: true})).
		Do(func(ctx context.Context) error {
			envStr := specificEnv
			if envStr == "" {
				// Need some env for package loading.
				envStr = "dev"
			}
			env, err := planning.LoadContext(locs.Root, envStr)
			if err != nil {
				return err
			}

			loc, bundle, err := loadBundleFromArgs(ctx, env, locs.Locs[0], nil)
			if err != nil {
				return err
			}

			key, err := parseKey(secretKey, string(loc.loc.PackageName))
			if err != nil {
				return err
			}

			key.EnvironmentName = specificEnv

			results, err := bundle.LookupValues(ctx, key)
			if err != nil {
				return err
			}

			out := console.Stdout(ctx)

			if len(results) == 1 && utf8.Valid(results[0].Value) {
				fmt.Fprintf(out, "%s\n", results[0].Value)
				return nil
			}

			for k, result := range results {
				if k > 0 {
					fmt.Fprintln(out)
				}

				secrets.DescribeKey(out, result.Key)

				if utf8.Valid(result.Value) {
					fmt.Fprintf(out, "\n\n  %s\n", result.Value)
				} else {
					fmt.Fprintf(out, " raw value:\n\n")
					if err := secrets.OutputBase64(text.NewIndentWriter(out, []byte("  ")), result.Value); err != nil {
						return err
					}
				}
			}

			return nil
		})
}
