// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package source

import (
	"context"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
)

func newYarnCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "yarn",
		Short: "Run yarn.",

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			return runNodejs(ctx, "yarn", args...)
		}),
	}

	return cmd
}