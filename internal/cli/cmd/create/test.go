// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package create

import (
	"context"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/frontend/cue"
	"namespacelabs.dev/foundation/internal/frontend/golang"
	"namespacelabs.dev/foundation/schema"
)

func newTestCmd() *cobra.Command {
	use := "test"
	cmd := &cobra.Command{
		Use:   use,
		Short: "Creates a stub for an e2e test.",
	}

	serverPkg := cmd.Flags().String("server", "", "Package name of the server.")
	servicePkg := cmd.Flags().String("service", "", "Package name of the service.")
	_ = cmd.MarkFlagRequired("server")
	_ = cmd.MarkFlagRequired("service")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		root, loc, err := targetPackage(ctx, args, use)
		if err != nil {
			return err
		}

		fmwk := schema.Framework_GO

		cueOpts := cue.GenTestOpts{
			ServerPkg: *serverPkg,
		}
		if err := cue.CreateTestScaffold(ctx, root.FS(), loc, cueOpts); err != nil {
			return err
		}

		switch fmwk {
		case schema.Framework_GO:
			goOpts := golang.GenTestOpts{ServicePkg: *servicePkg}
			if err := golang.CreateTestScaffold(ctx, root.FS(), loc, goOpts); err != nil {
				return err
			}
		}

		return nil
	})

	return cmd
}
