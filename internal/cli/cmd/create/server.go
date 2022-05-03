// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package create

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/languages/cue"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/module"
)

func newServerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Creates a server.",
		Args:  cobra.RangeArgs(0, 1),

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			root, loc, err := module.PackageAtArgs(ctx, args)
			if err != nil {
				return err
			}

			if loc.RelPath == "." {
				return fmt.Errorf("Cannot create server at workspace root. Please specify server location or run %s at the target directory.", colors.Bold("fn create server"))
			}

			var name string
			name = filepath.Base(loc.RelPath)

			framework := schema.Framework_GO_GRPC

			return cue.GenerateServer(ctx, root.FS(), loc, name, framework)

		}),
	}

	return cmd
}
