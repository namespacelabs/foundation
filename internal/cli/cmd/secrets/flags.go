// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package secrets

import (
	"context"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/parsing/module"
	"namespacelabs.dev/foundation/std/cfg"
)

func static(str string) *string {
	return &str
}

func envFromValue(cmd *cobra.Command, env *string) *cfg.Context {
	target := new(cfg.Context)

	pushParse(cmd, func(ctx context.Context, args []string) error {
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

func locationsFromArgs(cmd *cobra.Command, env *cfg.Context, opts ...fncobra.ParseLocationsOpts) *fncobra.Locations {
	target := new(fncobra.Locations)

	pushParse(cmd, func(ctx context.Context, args []string) error {
		locations, err := fncobra.ParseLocs(ctx, args, env, fncobra.MergeParseLocationOpts(opts))
		if err != nil {
			return err
		}
		*target = *locations
		return nil
	})

	return target
}

func pushParse(cmd *cobra.Command, handler func(ctx context.Context, args []string) error) {
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
