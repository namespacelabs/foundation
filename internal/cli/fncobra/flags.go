// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package fncobra

import (
	"context"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/parsing/module"
	"namespacelabs.dev/foundation/std/cfg"
)

func EnvFromValue(cmd *cobra.Command, env *string) *cfg.Context {
	target := new(cfg.Context)

	PushParse(cmd, func(ctx context.Context, args []string) error {
		root, err := module.FindRoot(ctx, ".")
		if err != nil {
			return err
		}

		sourceEnv := *env
		if sourceEnv == "" {
			sourceEnv = "dev"
		}

		env, err := cfg.LoadContext(root, sourceEnv)
		if err != nil {
			return err
		}

		*target = env
		return nil
	})

	return target
}

func LocationsFromArgs(cmd *cobra.Command, env *cfg.Context, opts ...ParseLocationsOpts) *Locations {
	target := new(Locations)

	PushParse(cmd, func(ctx context.Context, args []string) error {
		locations, err := ParseLocs(ctx, args, env, MergeParseLocationOpts(opts))
		if err != nil {
			return err
		}
		*target = *locations
		return nil
	})

	return target
}

func PushParse(cmd *cobra.Command, handler func(ctx context.Context, args []string) error) {
	previous := cmd.PreRunE
	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if previous != nil {
			if err := previous(cmd, args); err != nil {
				return err
			}
		}

		return handler(cmd.Context(), args)
	}
}
