// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package prepare

import (
	"context"

	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/build/registry"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/parsing/devhost"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/tasks"
)

func PrepareExistingK8s(env cfg.Context, contextName string, registry *registry.Registry) compute.Computable[*schema.DevHost_ConfigureEnvironment] {
	return compute.Map(
		tasks.Action("prepare.existing-k8s").HumanReadablef("Prepare a host-configured Kubernetes instance"),
		compute.Inputs().Indigestible("env", env),
		compute.Output{NotCacheable: true},
		func(ctx context.Context, _ compute.Resolved) (*schema.DevHost_ConfigureEnvironment, error) {
			var confs []proto.Message
			hostEnv := client.NewLocalHostEnv(contextName, env)
			confs = append(confs, hostEnv)
			if registry != nil {
				confs = append(confs, registry)
			}
			return devhost.MakeConfiguration(confs...)
		})
}
