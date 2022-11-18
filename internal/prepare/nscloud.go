// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package prepare

import (
	"context"
	"fmt"

	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/build/buildkit"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/parsing/devhost"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/tasks"
	"namespacelabs.dev/foundation/universe/nscloud/configuration"
)

func PrepareNewNamespaceCluster(env cfg.Context, machineType string, ephemeral bool, features []string) compute.Computable[*schema.DevHost_ConfigureEnvironment] {
	return compute.Map(
		tasks.Action("prepare.nscloud.new-cluster"),
		compute.Inputs().Proto("env", env.Environment()).Str("machineType", machineType).Strs("features", features).Indigestible("ephemeral", ephemeral),
		compute.Output{NotCacheable: true},
		func(ctx context.Context, _ compute.Resolved) (*schema.DevHost_ConfigureEnvironment, error) {
			cfg, err := api.CreateAndWaitCluster(ctx, machineType, ephemeral, env.Environment().Name, features)
			if err != nil {
				return nil, err
			}

			var messages []proto.Message
			messages = append(messages, &configuration.Cluster{ClusterId: cfg.ClusterId})

			if cfg.BuildCluster != nil {
				messages = append(messages, &buildkit.Overrides{
					HostedBuildCluster: &buildkit.HostedBuildCluster{
						ClusterId:  cfg.BuildCluster.Colocated.ClusterId,
						TargetPort: cfg.BuildCluster.Colocated.TargetPort,
					},
				})
			}

			c, err := devhost.MakeConfiguration(messages...)
			if err != nil {
				return nil, err
			}

			fmt.Fprintf(console.Stdout(ctx), "[✓] Create Kubernetes cluster in Namespace Cloud.\n")

			return c, nil
		})
}
