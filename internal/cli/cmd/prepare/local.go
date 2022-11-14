// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package prepare

import (
	"context"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/parsing/module"
	"namespacelabs.dev/foundation/internal/prepare"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
)

func newLocalCmd() *cobra.Command {
	var contextName string

	localCmd := &cobra.Command{
		Use:   "local",
		Short: "Prepares the local workspace for development or production.",

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			if contextName != "" {
				return fnerrors.New("to configure an existing cluster use `prepare existing`")
			}

			root, err := module.FindRoot(ctx, ".")
			if err != nil {
				return err
			}

			env, err := cfg.LoadContext(root, envRef)
			if err != nil {
				return err
			}

			if env.Environment().Purpose != schema.Environment_DEVELOPMENT {
				return fnerrors.BadInputError("only development environments are supported locally")
			}

			k8sconfig := prepare.PrepareK3d("ns", env)

			return collectPreparesAndUpdateDevhost(ctx, root, envRef, prepare.PrepareCluster(env, k8sconfig))
		}),
	}

	localCmd.Flags().StringVar(&contextName, "context", "", "If set, configures Namespace to use the specific context.")

	return localCmd
}
