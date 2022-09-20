// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package prepare

import (
	"context"

	"namespacelabs.dev/foundation/engine/compute"
	"namespacelabs.dev/foundation/runtime/kubernetes"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/planning"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func PrepareCluster(env planning.Context, confs ...compute.Computable[[]*schema.DevHost_ConfigureEnvironment]) []compute.Computable[[]*schema.DevHost_ConfigureEnvironment] {
	kube := instantiateKube(env, confs...)

	var prepares []compute.Computable[[]*schema.DevHost_ConfigureEnvironment]
	prepares = append(prepares, confs...)
	prepares = append(prepares, PrepareIngress(env, kube))

	return prepares
}

func instantiateKube(env planning.Context, confs ...compute.Computable[[]*schema.DevHost_ConfigureEnvironment]) compute.Computable[*kubernetes.Cluster] {
	return compute.Map(tasks.Action("prepare.kubernetes"),
		compute.Inputs().Computable("conf", compute.Transform("parse results", compute.Collect(tasks.Action("prepare.kubernetes.configs"), confs...),
			func(ctx context.Context, computed []compute.ResultWithTimestamp[[]*schema.DevHost_ConfigureEnvironment]) ([]*schema.DevHost_ConfigureEnvironment, error) {
				var result []*schema.DevHost_ConfigureEnvironment
				for _, conf := range computed {
					result = append(result, conf.Value...)
				}
				return result, nil
			})),
		compute.Output{},
		func(ctx context.Context, r compute.Resolved) (*kubernetes.Cluster, error) {
			computed, _ := compute.GetDepWithType[[]*schema.DevHost_ConfigureEnvironment](r, "conf")

			devhost := &schema.DevHost{Configure: computed.Value}

			config, err := planning.MakeConfigurationCompat(env, env.Workspace(), devhost, env.Environment())
			if err != nil {
				return nil, err
			}

			return kubernetes.ConnectToCluster(ctx, config)
		})
}
