// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package prepare

import (
	"context"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/engine/compute"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/prepare"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/std/planning"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/module"
)

func newLocalCmd() *cobra.Command {
	var contextName string

	localCmd := &cobra.Command{
		Use:   "local",
		Short: "Prepares the local workspace for development or production.",

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			root, err := module.FindRoot(ctx, ".")
			if err != nil {
				return err
			}

			env, err := planning.LoadContext(root, envRef)
			if err != nil {
				return err
			}

			if env.Environment().Purpose == schema.Environment_PRODUCTION && contextName == "" {
				return fnerrors.UsageError("Please also specify `--context`.",
					"Kubernetes context is required for preparing a production environment.")
			}

			sealedCtx := pkggraph.MakeSealedContext(env, workspace.NewPackageLoader(env).Seal())

			prepares := baseline(sealedCtx)

			k8sconfig := prepareK8s(ctx, env, contextName)
			prepares = append(prepares, prepare.PrepareCluster(env, k8sconfig)...)

			return collectPreparesAndUpdateDevhost(ctx, root, prepares)
		}),
	}

	localCmd.Flags().StringVar(&contextName, "context", "", "If set, configures Namespace to use the specific context.")

	return localCmd
}

func prepareK8s(ctx context.Context, env planning.Context, contextName string) compute.Computable[[]*schema.DevHost_ConfigureEnvironment] {
	if contextName != "" {
		return prepare.PrepareExistingK8s(env, contextName)
	}

	return prepare.PrepareK3d("fn", env)
}
