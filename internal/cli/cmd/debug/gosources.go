// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package debug

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/build"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/languages/golang"
)

func newGoSourcesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "go-sources",
		Short: "List go sources of a package.",
		Args:  cobra.RangeArgs(0, 1),

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			t, err := requireServer(ctx, args, "dev")
			if err != nil {
				return err
			}

			res, err := golang.ComputeSources(ctx, t.Module().Abs(), t, build.PlatformsOrOverrides(nil))
			if err != nil {
				return err
			}

			out := console.Stdout(ctx)

			for _, dep := range res.Deps {
				fmt.Fprintf(out, "dep: %s\n", dep)
			}

			for d, to := range res.DepTo {
				fmt.Fprintf(out, "%s --> %s\n", d, strings.Join(to, ", "))
			}

			for d, to := range res.GoFiles {
				fmt.Fprintf(out, "files: %s --> %s\n", d, strings.Join(to, ", "))
			}

			return nil
		}),
	}

	cmd.Flags().Var(build.BuildPlatformsVar{}, "build_platforms", "Allows the runtime to be instructed to build for a different set of platforms; by default we only build for the development host.")

	return cmd
}