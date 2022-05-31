// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package secrets

import (
	"context"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
)

func newAddReaderCmd() *cobra.Command {
	var keyID string
	var rawtext bool

	cmd := &cobra.Command{
		Use:   "add-reader",
		Short: "Adds a receipient to a secret bundle.",
		Args:  cobra.MaximumNArgs(1),
		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			loc, bundle, err := loadBundleFromArgs(ctx, args, nil)
			if err != nil {
				return err
			}

			if err := bundle.EnsureReader(keyID); err != nil {
				return err
			}

			return writeBundle(ctx, loc, bundle, !rawtext)
		}),
	}

	cmd.Flags().StringVar(&keyID, "key", "", "The key to add to the bundle.")
	cmd.Flags().BoolVar(&rawtext, "rawtext", rawtext, "If set to true, the bundle is not encrypted (use for testing purposes only).")
	_ = cmd.MarkFlagRequired("key")

	return cmd
}
