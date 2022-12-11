// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package prepare

import (
	"context"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/prepare"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
)

func newLocalCmd() *cobra.Command {
	var contextName string

	localCmd := &cobra.Command{
		Use:   "local",
		Short: "Prepares the local workspace for development or production.",

		RunE: runPrepare(func(ctx context.Context, env cfg.Context) ([]prepare.Stage, error) {
			if contextName != "" {
				return nil, fnerrors.New("to configure an existing cluster use `prepare existing`")
			}

			if env.Environment().Purpose != schema.Environment_DEVELOPMENT {
				return nil, fnerrors.BadInputError("only development environments are supported locally")
			}

			return []prepare.Stage{prepare.K3D("ns")}, nil
		}),
	}

	localCmd.Flags().StringVar(&contextName, "context", "", "If set, configures Namespace to use the specific context.")

	return localCmd
}
