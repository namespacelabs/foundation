// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package prepare

import (
	"context"

	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/providers/aws/eks"
	"namespacelabs.dev/foundation/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/devhost"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func PrepareEksCluster(clusterName string, env ops.Environment) compute.Computable[[]*schema.DevHost_ConfigureEnvironment] {
	return compute.Map(
		tasks.Action("prepare.eks-cluster-config").HumanReadablef("Prepare the EKS cluster configuration"),
		compute.Inputs().Str("clusterName", clusterName).Proto("env", env.Proto()),
		compute.Output{NotCacheable: true},
		func(ctx context.Context, _ compute.Resolved) ([]*schema.DevHost_ConfigureEnvironment, error) {
			hostEnv := &eks.EKSCluster{
				Name: clusterName,
			}
			c, err := devhost.MakeConfiguration(hostEnv)
			if err != nil {
				return nil, err
			}
			c.Purpose = env.Proto().GetPurpose()
			return []*schema.DevHost_ConfigureEnvironment{c}, nil
		})
}

func PrepareEksHostEnv(provider string, env ops.Environment) compute.Computable[[]*schema.DevHost_ConfigureEnvironment] {
	return compute.Map(
		tasks.Action("prepare.eks-host-env").HumanReadablef("Prepare the EKS host environment"),
		compute.Inputs().Str("provider", provider).Proto("env", env.Proto()),
		compute.Output{NotCacheable: true},
		func(ctx context.Context, _ compute.Resolved) ([]*schema.DevHost_ConfigureEnvironment, error) {
			hostEnv := &client.HostEnv{
				Provider: provider,
			}
			c, err := devhost.MakeConfiguration(hostEnv)
			if err != nil {
				return nil, err
			}
			c.Purpose = env.Proto().GetPurpose()
			return []*schema.DevHost_ConfigureEnvironment{c}, nil
		})
}
