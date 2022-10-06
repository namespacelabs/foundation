// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package prepare

import (
	"context"

	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/providers/nscloud"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/planning"
	"namespacelabs.dev/foundation/universe/nscloud/configuration"
	"namespacelabs.dev/foundation/workspace/devhost"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func PrepareNewNamespaceCluster(env planning.Context, machineType string, ephemeral bool) compute.Computable[[]*schema.DevHost_ConfigureEnvironment] {
	return compute.Map(
		tasks.Action("prepare.nscloud.new-cluster"),
		compute.Inputs().Proto("env", env.Environment()).Indigestible("foobar", "foobar"),
		compute.Output{NotCacheable: true},
		func(ctx context.Context, _ compute.Resolved) ([]*schema.DevHost_ConfigureEnvironment, error) {
			cfg, err := nscloud.CreateAndWaitCluster(ctx, machineType, ephemeral, env.Environment().Name)
			if err != nil {
				return nil, err
			}

			c, err := devhost.MakeConfiguration(&configuration.Cluster{ClusterId: cfg.ClusterId})
			if err != nil {
				return nil, err
			}

			c.Name = env.Environment().Name
			return []*schema.DevHost_ConfigureEnvironment{c}, nil
		})
}
