// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/build/buildkit"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/compute/cache"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/tui"
	"namespacelabs.dev/foundation/internal/parsing/devhost"
	"namespacelabs.dev/foundation/internal/unprepare"
)

func NewUnprepareCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unprepare",
		Short: "Removes Namespace-created docker containers and user-level caches.",
		Args:  cobra.NoArgs,

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			result, err := tui.Ask(ctx, "Do you want to remove all of Namespace locally managed resources?",
				`If you've run Namespace before, various resources were setup in your
workstation, including a result cache, but most importantly a series of
containers running within your Docker instance.

Do you wish to remove these?

Type "unprepare" for them to be removed.`, "")

			if result != "unprepare" {
				return context.Canceled
			}

			if err != nil {
				return err
			}

			// Remove k3d cluster(s) and registry(ies).
			if err := unprepare.UnprepareK3d(ctx); err != nil {
				return err
			}

			// Stop and remove the builtkit daemon container.
			if err := buildkit.RemoveBuildkitd(ctx); err != nil {
				return err
			}

			// Prune cached build artifacts and command history artifacts.
			if err := cache.Prune(ctx); err != nil {
				return err
			}

			fmt.Fprintf(console.Stdout(ctx), "The contents of your %q are no longer valid.\n", devhost.DevHostFilename)

			return nil
		}),
	}

	return cmd
}
