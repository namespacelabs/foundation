// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package secrets

import (
	"context"
	"fmt"
	"io/ioutil"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console/tui"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/secrets"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
)

func newSetCmd() *cobra.Command {
	var secretKey, keyID, fromFile, specificEnv string
	var rawtext bool
	var locs fncobra.Locations

	return fncobra.
		Cmd(&cobra.Command{
			Use:   "set",
			Short: "Sets the specified secret value.",
			Args:  cobra.MaximumNArgs(1),
		}).
		WithFlags(func(cmd *cobra.Command) {
			cmd.Flags().StringVar(&secretKey, "secret", "", "The secret key, in {package_name}:{name} format.")
			cmd.Flags().StringVar(&keyID, "key", "", "Use this specific key identity when creating a new bundle.")
			cmd.Flags().StringVar(&fromFile, "from_file", "", "Load the file contents as the secret value.")
			cmd.Flags().BoolVar(&rawtext, "rawtext", rawtext, "If set to true, the bundle is not encrypted (use for testing purposes only).")
			cmd.Flags().StringVar(&specificEnv, "env", "", "If set, only sets the specified secret for the named environment (e.g. dev, or prod).")
			_ = cmd.MarkFlagRequired("secret")
		}).
		With(fncobra.ParseLocations(&locs, &fncobra.ParseLocationsOpts{RequireSingle: true})).
		Do(func(ctx context.Context) error {
			loc, bundle, err := loadBundleFromArgs(ctx, locs.Locs[0], func(ctx context.Context) (*secrets.Bundle, error) {
				return secrets.NewBundle(ctx, keyID)
			})
			if err != nil {
				return err
			}

			key, err := parseKey(secretKey)
			if err != nil {
				return err
			}

			if specificEnv != "" {
				if _, err := provision.RequireEnv(loc.root, specificEnv); err != nil {
					return err
				}

				key.EnvironmentName = specificEnv
			}

			if _, err := workspace.NewPackageLoader(loc.root).LoadByName(ctx, schema.PackageName(key.PackageName)); err != nil {
				return err
			}

			var value []byte
			if fromFile != "" {
				value, err = ioutil.ReadFile(fromFile)
				if err != nil {
					return fnerrors.BadInputError("%s: failed to load: %w", fromFile, err)
				}
			} else {
				valueStr, err := tui.Ask(ctx, "Set a new secret value", fmt.Sprintf("Package: %s\nKey: %q\n\n%s", key.PackageName, key.Key, lipgloss.NewStyle().Faint(true).Render("Note: for multi-line input, use the --from_file flag.")), "Value")
				if err != nil {
					return err
				}
				if valueStr == "" {
					return fnerrors.New("no value provided, skipping")
				}
				value = []byte(valueStr)
			}

			bundle.Set(key, value)

			return writeBundle(ctx, loc, bundle, !rawtext)
		})
}
