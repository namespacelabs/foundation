// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package fncobra

import (
	"context"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/std/planning"
	"namespacelabs.dev/foundation/workspace/module"
)

// Deprecated, use "ParseEnv"/"FixedEnv" instead.
func CmdWithEnv(cmd *cobra.Command, f func(context.Context, planning.Context, []string) error) *cobra.Command {
	var envRef string

	cmd.Flags().StringVar(&envRef, "env", "dev", "The environment to access (as defined in the workspace).")

	cmd.RunE = RunE(func(ctx context.Context, args []string) error {
		root, err := module.FindRoot(ctx, ".")
		if err != nil {
			return err
		}

		env, err := planning.LoadContext(root, envRef)
		if err != nil {
			return err
		}

		return f(ctx, env, args)
	})

	return cmd
}

type parseEnv struct {
	envOut *planning.Context
	envRef string
}

func ParseEnv(envOut *planning.Context) ArgsParser {
	return &parseEnv{envOut: envOut}
}

// HardcodeEnv is a temporary facility to trigger context loading with a predefined environment name.
func HardcodeEnv(envOut *planning.Context, env string) ArgsParser {
	return &parseEnv{envOut: envOut, envRef: env}
}

func (p *parseEnv) AddFlags(cmd *cobra.Command) {
	if p.envRef == "" {
		cmd.Flags().StringVar(&p.envRef, "env", "dev", "The environment to access (as defined in the workspace).")
	}
}

func (p *parseEnv) Parse(ctx context.Context, args []string) error {
	if p.envOut == nil {
		return fnerrors.InternalError("envOut must be set")
	}

	root, err := module.FindRoot(ctx, ".")
	if err != nil {
		return err
	}

	env, err := planning.LoadContext(root, p.envRef)
	if err != nil {
		return err
	}

	*p.envOut = env

	return nil
}
