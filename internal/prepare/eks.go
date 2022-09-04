// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package prepare

import (
	"context"

	"namespacelabs.dev/foundation/build/registry"
	"namespacelabs.dev/foundation/internal/planning"
	"namespacelabs.dev/foundation/providers/aws/eks"
	"namespacelabs.dev/foundation/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/devhost"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func PrepareEksCluster(env planning.Context, clusterName string) compute.Computable[[]*schema.DevHost_ConfigureEnvironment] {
	return compute.Map(
		tasks.Action("prepare.eks-cluster-config").HumanReadablef("Prepare the EKS cluster configuration"),
		compute.Inputs().Str("clusterName", clusterName).Proto("env", env.Environment()),
		compute.Output{NotCacheable: true},
		func(ctx context.Context, _ compute.Resolved) ([]*schema.DevHost_ConfigureEnvironment, error) {
			k8sHostEnv := &client.HostEnv{
				Provider: "aws/eks",
			}
			eksCluster := &eks.EKSCluster{
				Name: clusterName,
			}
			registryProvider := &registry.Provider{
				Provider: "aws/ecr",
			}

			c, err := devhost.MakeConfiguration(k8sHostEnv, eksCluster, registryProvider)
			if err != nil {
				return nil, err
			}
			c.Name = env.Environment().Name
			return []*schema.DevHost_ConfigureEnvironment{c}, nil
		})
}
