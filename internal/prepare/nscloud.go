// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package prepare

import (
	"context"
	"fmt"

	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/executor"
	"namespacelabs.dev/foundation/internal/parsing/devhost"
	"namespacelabs.dev/foundation/internal/providers/nscloud"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/tasks"
	"namespacelabs.dev/foundation/universe/nscloud/configuration"
)

func PrepareNewNamespaceCluster(env cfg.Context, machineType string, ephemeral, withBuild bool) compute.Computable[*schema.DevHost_ConfigureEnvironment] {
	return compute.Map(
		tasks.Action("prepare.nscloud.new-cluster"),
		compute.Inputs().Proto("env", env.Environment()).Str("machineType", machineType).Bool("withBuild", withBuild).Indigestible("ephemeral", ephemeral),
		compute.Output{NotCacheable: true},
		func(ctx context.Context, _ compute.Resolved) (*schema.DevHost_ConfigureEnvironment, error) {
			eg := executor.New(ctx, "prepare-new-cluster")

			var mainMessages, buildMessages []proto.Message

			eg.Go(func(ctx context.Context) error {
				cfg, err := api.CreateAndWaitCluster(ctx, api.Endpoint, machineType, ephemeral, env.Environment().Name, nil)
				if err != nil {
					return err
				}
				mainMessages = append(mainMessages, &configuration.Cluster{ClusterId: cfg.ClusterId})
				return nil
			})

			if withBuild {
				eg.Go(func(ctx context.Context) error {
					msg, err := nscloud.EnsureBuildCluster(ctx, api.Endpoint)
					if err != nil {
						return err
					}

					buildMessages = append(buildMessages, msg)
					return nil
				})
			}

			if err := eg.Wait(); err != nil {
				return nil, err
			}

			c, err := devhost.MakeConfiguration(append(mainMessages, buildMessages...)...)
			if err != nil {
				return nil, err
			}

			fmt.Fprintf(console.Stdout(ctx), "[✓] Create Kubernetes cluster in Namespace Cloud.\n")

			return c, nil
		})
}
