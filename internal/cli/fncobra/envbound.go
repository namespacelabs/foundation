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

func CmdWithEnv(cmd *cobra.Command, f func(context.Context, provision.Env, []string) error) *cobra.Command {
	var envRef string

	cmd.Flags().StringVar(&envRef, "env", "dev", "The environment to access (as defined in the workspace).")

	storedrun.SetupFlags(cmd.Flags())

	cmd.RunE = RunE(func(ctx context.Context, args []string) error {
		s, err := storedrun.Check()
		if err != nil {
			return err
		}

		root, err := module.FindRoot(ctx, ".")
		if err != nil {
			return err
		}

		env, err := provision.RequireEnv(root, envRef)
		if err != nil {
			return err
		}

		return s.Output(ctx, env, f(ctx, env, args))
	})

	return cmd
}
