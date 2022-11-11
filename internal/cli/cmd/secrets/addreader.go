// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package secrets

import (
	"context"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
)

func newAddReaderCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add-reader --key {public-key} [server]",
		Short: "Adds a receipient to a secret bundle.",
		Args:  cobra.MaximumNArgs(1),
	}

	keyID := cmd.Flags().String("key", "", "The reader public key to add to the bundle.")
	rawtext := cmd.Flags().Bool("rawtext", false, "If set to true, the bundle is not encrypted (use for testing purposes only).")
	_ = cmd.MarkFlagRequired("key")
	env := envFromValue(cmd, static("dev"))
	locs := locationsFromArgs(cmd, env)
	loc, bundle := bundleFromArgs(cmd, env, locs, nil)

	return fncobra.With(cmd, func(ctx context.Context) error {
		if err := bundle.EnsureReader(*keyID); err != nil {
			return err
		}

		return writeBundle(ctx, loc, bundle, !*rawtext)
	})
}
