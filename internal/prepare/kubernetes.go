// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package prepare

import (
	"context"

	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/tasks"
)

func PrepareCluster(env cfg.Context, confs ...compute.Computable[*schema.DevHost_ConfigureEnvironment]) compute.Computable[*schema.DevHost_ConfigureEnvironment] {
	kube := instantiateKube(env, confs...)

	var prepares []compute.Computable[*schema.DevHost_ConfigureEnvironment]
	prepares = append(prepares, confs...)
	prepares = append(prepares, PrepareOrchestrator(env, kube, confs...))
	prepares = append(prepares, PrepareIngress(env, kube))

	c := compute.Collect(tasks.Action("prepare.collect-configs"), prepares...)

	return compute.Map(tasks.Action("prepare.merge-configs"), compute.Inputs().Computable("configs", c), compute.Output{NotCacheable: true}, func(ctx context.Context, r compute.Resolved) (*schema.DevHost_ConfigureEnvironment, error) {
		configs := compute.MustGetDepValue(r, c, "configs")
		merged := &schema.DevHost_ConfigureEnvironment{}
		for _, config := range configs {
			if config.Value != nil {
				merged.Configuration = append(merged.Configuration, config.Value.Configuration...)
			}
		}
		return merged, nil
	})
}

func mergeConfigs(ctx context.Context, computed []compute.ResultWithTimestamp[*schema.DevHost_ConfigureEnvironment]) (*schema.DevHost, error) {
	devhost := &schema.DevHost{}
	for _, conf := range computed {
		devhost.Configure = append(devhost.Configure, conf.Value)
	}
	return devhost, nil
}

func instantiateKube(env cfg.Context, confs ...compute.Computable[*schema.DevHost_ConfigureEnvironment]) compute.Computable[*kubernetes.Cluster] {
	return compute.Map(tasks.Action("prepare.kubernetes"),
		compute.Inputs().Computable("conf", compute.Transform("parse results",
			compute.Collect(tasks.Action("prepare.kubernetes.configs"), confs...), mergeConfigs)),
		compute.Output{},
		func(ctx context.Context, r compute.Resolved) (*kubernetes.Cluster, error) {
			devhost, _ := compute.GetDepWithType[*schema.DevHost](r, "conf")

			config, err := cfg.MakeConfigurationCompat(env, env.Workspace(), devhost.Value, env.Environment())
			if err != nil {
				return nil, err
			}

			return kubernetes.ConnectToCluster(ctx, config)
		})
}
