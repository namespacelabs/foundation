// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package create

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/workspace/source/codegen"
)

func newServiceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "service",
		Short: "Creates a service.",

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			root, loc, err := targetPackage(ctx, args, "server")
			if err != nil {
				return err
			}

			// TODO add file creation

			return codegen.ForLocations(ctx, root, []fnfs.Location{loc}, func(e codegen.GenerateError) {
				w := console.Stderr(ctx)
				fmt.Fprintf(w, "%s: %s failed:\n", e.PackageName, e.What)
				fnerrors.Format(w, true, e.Err)
			})

		}),
	}

	return cmd
}
