// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package debug

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/workspace/dirs"
)

func newFindConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "find-config",
		Short: "Prints location of config files.",
		Args:  cobra.ArbitraryArgs,

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			fnDir, err := dirs.Config()
			if err != nil {
				return err
			}

			fmt.Fprintf(console.Stdout(ctx), "%s\n", fnDir)
			return nil
		}),
	}

	return cmd
}
