// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package prepare

import (
	"context"

	"namespacelabs.dev/foundation/engine/compute"
	"namespacelabs.dev/foundation/engine/ops"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/runtime/kubernetes"
	"namespacelabs.dev/foundation/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/runtime/kubernetes/networking/ingress"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/planning"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func PrepareIngressFromHostConfig(env planning.Context, k8sconfig compute.Computable[*client.HostConfig]) compute.Computable[[]*schema.DevHost_ConfigureEnvironment] {
	return PrepareIngress(env, compute.Transform("create k8s runtime", k8sconfig, func(ctx context.Context, cfg *client.HostConfig) (*kubernetes.Cluster, error) {
		return kubernetes.ConnectToConfig(ctx, cfg)
	}))
}

func PrepareIngress(env planning.Context, kube compute.Computable[*kubernetes.Cluster]) compute.Computable[[]*schema.DevHost_ConfigureEnvironment] {
	return compute.Map(
		tasks.Action("prepare.ingress").HumanReadablef("Deploying the Kubernetes ingress controller"),
		compute.Inputs().Str("kind", "ingress").Computable("runtime", kube).Proto("env", env.Environment()).Proto("workspace", env.Workspace().Proto()),
		compute.Output{NotCacheable: true},
		func(ctx context.Context, deps compute.Resolved) ([]*schema.DevHost_ConfigureEnvironment, error) {
			kube := compute.MustGetDepValue(deps, kube, "runtime")

			if err := PrepareIngressInKube(ctx, env, kube); err != nil {
				return nil, err
			}

			// The ingress produces no unique configuration.
			return nil, nil
		})
}

func PrepareIngressInKube(ctx context.Context, env planning.Context, kube *kubernetes.Cluster) error {
	for _, lbl := range kube.PreparedClient().Configuration.Labels {
		if lbl.Name == ingress.LblNameStatus {
			if lbl.Value == "installed" {
				return nil
			}
			break
		}
	}

	state, err := kube.PrepareCluster(ctx)
	if err != nil {
		return err
	}

	g := ops.NewPlan(state.Definitions...)

	if err := ops.Execute(ctx, env, "ingress.deploy", g, nil, runtime.ClusterInjection.With(kube)); err != nil {
		return err
	}

	return nil
}
