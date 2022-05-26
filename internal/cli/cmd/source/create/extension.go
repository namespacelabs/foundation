// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package create

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/frontend/cue"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/module"
)

func targetPackage(ctx context.Context, args []string, typ string) (*workspace.Root, fnfs.Location, error) {
	root, loc, err := module.PackageAtArgs(ctx, args)
	if err != nil {
		return nil, fnfs.Location{}, err
	}

	if loc.RelPath == "." {
		cmd := fmt.Sprintf("fn create %s", typ)
		return nil, fnfs.Location{}, fmt.Errorf("cannot create %s at workspace root. Please specify %s location or run %s at the target directory",
			typ, typ, colors.Bold(cmd))
	}

	return root, loc, nil
}

func newExtensionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "extension",
		Short: "Creates an extension.",

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			root, loc, err := targetPackage(ctx, args, "extension")
			if err != nil {
				return err
			}

			if err := cue.CreateExtensionScaffold(ctx, root.FS(), loc); err != nil {
				return err
			}

			return codegenNode(ctx, root, loc)
		}),
	}

	return cmd
}
