// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package secrets

import (
	"context"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/std/cfg"
)

func newInfoCmd() *cobra.Command {
	var (
		locs fncobra.Locations
		env  cfg.Context
	)

	return fncobra.Cmd(
		&cobra.Command{
			Use:   "info [server]",
			Short: "Describes the contents of the specified server's secrets archive.",
			Args:  cobra.MaximumNArgs(1),
		}).
		With(
			fncobra.HardcodeEnv(&env, "dev"),
			fncobra.ParseLocations(&locs, &env)).
		Do(func(ctx context.Context) error {
			_, bundle, err := loadBundleFromArgs(ctx, env, locs, nil)
			if err != nil {
				return err
			}

			bundle.DescribeTo(console.Stdout(ctx))
			return nil
		})
}
