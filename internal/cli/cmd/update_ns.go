// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cmd

import (
	"context"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/cli/nsboot"
)

func NewUpdateNSCmd() *cobra.Command {
	// This command is installed in the ns binary as a placeholder for the
	// one implemented in nsboot. This makes it show up in `ns help`.
	cmd := &cobra.Command{
		Use:     "self-update",
		Short:   "Checks and downloads updates for the ns command.",
		Aliases: []string{"update-ns"},

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			return nsboot.ForceUpdate(ctx)
		}),
	}

	return cmd
}
