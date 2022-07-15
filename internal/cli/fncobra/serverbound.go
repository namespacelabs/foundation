// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package fncobra

import (
	"context"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/module"
)

func CmdWithServer(cmd *cobra.Command, f func(context.Context, provision.Server) error) *cobra.Command {
	var envRef string
	var packageName string

	cmd.Flags().StringVar(&envRef, "env", "dev", "The environment to access (as defined in the workspace).")
	cmd.Flags().StringVar(&packageName, "package_name", "", "Specify the server by package name instead.")

	cmd.RunE = RunE(func(ctx context.Context, args []string) error {
		var root *workspace.Root
		var pkg schema.PackageName

		if packageName != "" {
			var err error
			root, err = module.FindRoot(ctx, ".")
			if err != nil {
				return err
			}

			pkg = schema.PackageName(packageName)
		} else {
			detectedRoot, loc, err := module.PackageAtArgs(ctx, args)
			if err != nil {
				return err
			}

			root = detectedRoot
			pkg = loc.AsPackageName()
		}

		env, err := provision.RequireEnv(root, envRef)
		if err != nil {
			return err
		}

		server, err := env.RequireServer(ctx, pkg)
		if err != nil {
			return err
		}

		return f(ctx, server)
	})

	return cmd
}
