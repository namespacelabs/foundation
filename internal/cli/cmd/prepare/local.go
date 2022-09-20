// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package prepare

import (
	"context"

	"github.com/spf13/cobra"
	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/engine/compute"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/prepare"
	"namespacelabs.dev/foundation/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/planning"
	"namespacelabs.dev/foundation/workspace/devhost"
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

			prepares := baseline(env)

			k8sconfig := prepareK8s(ctx, env, contextName)
			local := localK8sConfiguration(env, k8sconfig)
			prepares = append(prepares, prepare.PrepareCluster(env, local)...)

			return collectPreparesAndUpdateDevhost(ctx, root, prepares)
		}),
	}

	localCmd.Flags().StringVar(&contextName, "context", "", "If set, configures Namespace to use the specific context.")

	return localCmd
}

func prepareK8s(ctx context.Context, env planning.Context, contextName string) compute.Computable[*client.HostConfig] {
	if contextName != "" {
		return prepare.PrepareExistingK8s(env, prepare.WithK8sContextName(contextName))
	}

	return prepare.PrepareK3d("fn", env)
}

func localK8sConfiguration(env planning.Context, hostConfig compute.Computable[*client.HostConfig]) compute.Computable[[]*schema.DevHost_ConfigureEnvironment] {
	return compute.Transform("create local config", hostConfig, func(ctx context.Context, k8sconfigval *client.HostConfig) ([]*schema.DevHost_ConfigureEnvironment, error) {
		var messages []proto.Message

		registry := k8sconfigval.Registry()
		if registry != nil {
			messages = append(messages, registry)
		}

		hostEnv := k8sconfigval.ClientHostEnv()
		if hostEnv != nil {
			messages = append(messages, hostEnv)
		}

		c, err := devhost.MakeConfiguration(messages...)
		if err != nil {
			return nil, err
		}
		c.Name = env.Environment().GetName()
		c.Runtime = "kubernetes"

		var confs []*schema.DevHost_ConfigureEnvironment
		confs = append(confs, c)
		return confs, nil
	})
}
