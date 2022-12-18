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
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console/tui"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/internal/secrets/localsecrets"
	"namespacelabs.dev/foundation/schema"
)

func newSetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set --secret {package_name}:{name} [--from_file <path>] [server]",
		Short: "Sets the specified secret value.",
		Args:  cobra.MaximumNArgs(1),
	}

	secretKey := cmd.Flags().String("secret", "", "The secret key, in {package_name}:{name} format.")
	specificEnv := cmd.Flags().String("env", "", "If set, matches specified secret with the named environment (e.g. dev, or prod).")
	keyID := cmd.Flags().String("key", "", "Use this specific key identity when creating a new bundle.")
	fromFile := cmd.Flags().String("from_file", "", "Load the file contents as the secret value.")
	rawtext := cmd.Flags().Bool("rawtext", false, "If set to true, the bundle is not encrypted (use for testing purposes only).")
	_ = cmd.MarkFlagRequired("secret")

	env := envFromValue(cmd, specificEnv)
	locs := locationsFromArgs(cmd, env)
	loc, bundle := bundleFromArgs(cmd, env, locs, func(ctx context.Context) (*localsecrets.Bundle, error) {
		return localsecrets.NewBundle(ctx, *keyID)
	})

	return fncobra.With(cmd, func(ctx context.Context) error {
		key, err := parseKey(*secretKey)
		if err != nil {
			return err
		}

		key.EnvironmentName = *specificEnv

		if _, err := parsing.NewPackageLoader(*env).LoadByName(ctx, schema.PackageName(key.PackageName)); err != nil {
			return err
		}

		var value []byte
		if *fromFile != "" {
			value, err = os.ReadFile(*fromFile)
			if err != nil {
				return fnerrors.BadInputError("%s: failed to load: %w", *fromFile, err)
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

		return writeBundle(ctx, loc, bundle, !*rawtext)
	})
}
