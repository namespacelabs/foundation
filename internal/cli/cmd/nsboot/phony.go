// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package nsboot

import (
	"context"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/fnerrors"
)

func NewPhoneUpdateNSCmd() *cobra.Command {
	// This command is installed in the ns binary as a placeholder for the
	// one implemented in nsboot. This makes it show up in `ns help`.
	cmd := &cobra.Command{
		Use:   "update-ns",
		Short: "Checks and downloads updates for the ns command.",

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			v, err := GetBootVersion()
			if err != nil {
				return fnerrors.InternalError("Unexpected ns version: %w. "+
					"Please reinstall ns via supported means as documented at https://get.namespace.so.", err)
			}
			if v != nil {
				// NSBOOT_VERSION in the environment, but update-ns not intercepted??
				return fnerrors.InternalError("Failed to invoke update logic. " +
					"Please reinstall ns via supported means as documented at https://get.namespace.so.")
			}
			return fnerrors.UsageError(
				"Reinstall ns via supported means as documented at https://get.namespace.so.",
				"Automatic updates are only supported via nsboot helper binary.")
		}),
	}

	return cmd
}
