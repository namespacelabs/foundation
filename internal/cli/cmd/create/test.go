// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package create

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/frontend/scaffold"
	"namespacelabs.dev/foundation/schema"
)

func newTestCmd() *cobra.Command {
	var (
		targetPkg  targetPkg
		serverPkg  string
		servicePkg string
	)

	return fncobra.
		Cmd(&cobra.Command{
			Use:   "test [path/to/package] --server {path/to/server} --service {path/to/service}",
			Short: "Creates a stub for an e2e test.",
		}).
		WithFlags(func(flags *pflag.FlagSet) {
			flags.StringVar(&serverPkg, "server", "", "Package name of the server.")
			flags.StringVar(&servicePkg, "service", "", "Package name of the service.")
			_ = cobra.MarkFlagRequired(flags, "server")
			_ = cobra.MarkFlagRequired(flags, "service")
		}).
		With(parseTargetPkgWithDeps(&targetPkg, "test")...).
		Do(func(ctx context.Context) error {

			fmwk := schema.Framework_GO

			cueOpts := scaffold.GenTestOpts{
				ServerPkg: serverPkg,
			}
			if err := scaffold.CreateTestScaffold(ctx, targetPkg.Root.ReadWriteFS(), targetPkg.Location, cueOpts); err != nil {
				return err
			}

			switch fmwk {
			case schema.Framework_GO:
				goOpts := scaffold.GenGoTestOpts{ServicePkg: servicePkg}
				if err := scaffold.CreateGoTestScaffold(ctx, targetPkg.Root.ReadWriteFS(), targetPkg.Location, goOpts); err != nil {
					return err
				}
			}

			return nil
		})
}
