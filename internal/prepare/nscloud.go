// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package prepare

import (
	"context"

	"namespacelabs.dev/foundation/build/registry"
	"namespacelabs.dev/foundation/providers/nscloud"
	"namespacelabs.dev/foundation/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/planning"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/devhost"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func PrepareNewNamespaceCluster(env planning.Context) compute.Computable[[]*schema.DevHost_ConfigureEnvironment] {
	return compute.Map(
		tasks.Action("prepare.nscloud.new-cluster"),
		compute.Inputs().Proto("env", env.Environment()).Indigestible("foobar", "foobar"),
		compute.Output{NotCacheable: true},
		func(ctx context.Context, _ compute.Resolved) ([]*schema.DevHost_ConfigureEnvironment, error) {
			cfg, err := nscloud.CreateClusterForEnv(ctx, env.Configuration(), false)
			if err != nil {
				return nil, err
			}

			k8sHostEnv := &client.HostEnv{
				Provider: "nscloud",
			}

			registryProvider := &registry.Provider{
				Provider: "nscloud",
			}

			prebuilt := &nscloud.PrebuiltCluster{
				ClusterId: cfg.ClusterId,
			}

			c, err := devhost.MakeConfiguration(k8sHostEnv, prebuilt, registryProvider)
			if err != nil {
				return nil, err
			}

			c.Name = env.Environment().Name
			return []*schema.DevHost_ConfigureEnvironment{c}, nil
		})
}
