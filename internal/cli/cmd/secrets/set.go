// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package secrets

import (
	"context"
	"fmt"
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console/tui"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/internal/secrets"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
)

func newSetCmd() *cobra.Command {
	var (
		locLoadingEnv                           cfg.Context
		secretKey, keyID, fromFile, specificEnv string
		rawtext                                 bool
		locs                                    fncobra.Locations
	)

	return fncobra.
		Cmd(&cobra.Command{
			Use:   "set {path/to/server} --secret {package_name}:{name} [--from_file <path>]",
			Short: "Sets the specified secret value.",
			Args:  cobra.MaximumNArgs(1),
		}).
		WithFlags(func(flags *pflag.FlagSet) {
			flags.StringVar(&secretKey, "secret", "", "The secret key, in {package_name}:{name} format.")
			flags.StringVar(&keyID, "key", "", "Use this specific key identity when creating a new bundle.")
			flags.StringVar(&fromFile, "from_file", "", "Load the file contents as the secret value.")
			flags.BoolVar(&rawtext, "rawtext", rawtext, "If set to true, the bundle is not encrypted (use for testing purposes only).")
			flags.StringVar(&specificEnv, "env", "", "If set, only sets the specified secret for the named environment (e.g. dev, or prod).")
			_ = cobra.MarkFlagRequired(flags, "secret")
		}).
		With(
			fncobra.HardcodeEnv(&locLoadingEnv, "dev"),
			fncobra.ParseLocations(&locs, &locLoadingEnv, fncobra.ParseLocationsOpts{RequireSingle: true})).
		Do(func(ctx context.Context) error {
			envStr := specificEnv
			if envStr == "" {
				// Need some env for package loading.
				envStr = "dev"
			}
			env, err := cfg.LoadContext(locs.Root, envStr)
			if err != nil {
				return err
			}

			loc, bundle, err := loadBundleFromArgs(ctx, env, locs.Locs[0], func(ctx context.Context) (*secrets.Bundle, error) {
				return secrets.NewBundle(ctx, keyID)
			})
			if err != nil {
				return err
			}

			key, err := parseKey(secretKey, string(loc.loc.PackageName))
			if err != nil {
				return err
			}

			key.EnvironmentName = specificEnv

			if _, err := parsing.NewPackageLoader(env).LoadByName(ctx, schema.PackageName(key.PackageName)); err != nil {
				return err
			}

			var value []byte
			if fromFile != "" {
				value, err = os.ReadFile(fromFile)
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
