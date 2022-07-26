// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package fncobra

import (
	"context"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/workspace/module"
)

// Deprecated
func CmdWithEnv(cmd *cobra.Command, f func(context.Context, provision.Env, []string) error) *cobra.Command {
	var envRef string

	cmd.Flags().StringVar(&envRef, "env", "dev", "The environment to access (as defined in the workspace).")

	cmd.RunE = RunE(func(ctx context.Context, args []string) error {
		root, err := module.FindRoot(ctx, ".")
		if err != nil {
			return err
		}

		env, err := provision.RequireEnv(root, envRef)
		if err != nil {
			return err
		}

		return f(ctx, env, args)
	})

	return cmd
}

type EnvParser struct {
	envOut *provision.Env
	envRef string
}

func NewEnvParser(envOut *provision.Env) *EnvParser {
	return &EnvParser{envOut: envOut}
}

func (p *EnvParser) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&p.envRef, "env", "dev", "The environment to access (as defined in the workspace).")
}

func (p *EnvParser) Parse(ctx context.Context, args []string) error {
	if p.envOut == nil {
		return fnerrors.InternalError("envOut must be set")
	}

	root, err := module.FindRoot(ctx, ".")
	if err != nil {
		return err
	}

	env, err := provision.RequireEnv(root, p.envRef)
	if err != nil {
		return err
	}

	*p.envOut = env

	return nil
}
