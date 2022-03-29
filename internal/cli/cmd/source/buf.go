// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package source

import (
	"context"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/sdk/buf"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/runtime/rtypes"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/module"
)

func newBufCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "buf",
		Short: "Run buf.build.",

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			root, err := module.FindRoot(ctx, ".")
			if err != nil {
				return err
			}

			env, err := provision.RequireEnv(root, "dev")
			if err != nil {
				return err
			}

			pl := workspace.NewPackageLoader(root)

			return buf.BuildAndrun(ctx, env, root, pl, rtypes.StdIO(ctx), args...)
		}),
	}

	return cmd
}