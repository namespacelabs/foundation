// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package prepare

import (
	"context"

	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/parsing/devhost"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/tasks"
)

func PrepareExistingK8s(env cfg.Context, contextName string) compute.Computable[[]*schema.DevHost_ConfigureEnvironment] {
	return compute.Map(
		tasks.Action("prepare.existing-k8s").HumanReadablef("Prepare a host-configured Kubernetes instance"),
		compute.Inputs().Proto("env", env.Environment()),
		compute.Output{NotCacheable: true},
		func(ctx context.Context, _ compute.Resolved) ([]*schema.DevHost_ConfigureEnvironment, error) {
			hostEnv := client.NewLocalHostEnv(contextName, env)
			c, err := devhost.MakeConfiguration(hostEnv)
			if err != nil {
				return nil, err
			}
			c.Name = env.Environment().Name
			return []*schema.DevHost_ConfigureEnvironment{c}, nil
		})
}
