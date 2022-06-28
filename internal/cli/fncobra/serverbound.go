// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package fncobra

import (
	"context"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/storedrun"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/workspace/module"
)

func CmdWithServer(cmd *cobra.Command, f func(context.Context, provision.Server) error) *cobra.Command {
	var envRef string

	cmd.Flags().StringVar(&envRef, "env", "dev", "The environment to access (as defined in the workspace).")

	storedrun.SetupFlags(cmd.Flags())

	cmd.RunE = RunE(func(ctx context.Context, args []string) error {
		s, err := storedrun.Check()
		if err != nil {
			return err
		}

		root, loc, err := module.PackageAtArgs(ctx, args)
		if err != nil {
			return err
		}

		env, err := provision.RequireEnv(root, envRef)
		if err != nil {
			return err
		}

		server, err := env.RequireServer(ctx, loc.AsPackageName())
		if err != nil {
			return err
		}

		return s.Output(ctx, env, f(ctx, server))
	})

	return cmd
}
