// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package prepare

import (
	"context"
	"fmt"

	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes/networking/ingress"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/execution"
	"namespacelabs.dev/foundation/std/tasks"
)

func PrepareIngress(env cfg.Context, kube compute.Computable[*kubernetes.Cluster]) compute.Computable[*schema.DevHost_ConfigureEnvironment] {
	return compute.Map(
		tasks.Action("prepare.ingress").HumanReadablef("Deploying the Kubernetes ingress controller"),
		compute.Inputs().Str("kind", "ingress").Computable("runtime", kube).Proto("env", env.Environment()).Proto("workspace", env.Workspace().Proto()),
		compute.Output{NotCacheable: true},
		func(ctx context.Context, deps compute.Resolved) (*schema.DevHost_ConfigureEnvironment, error) {
			kube := compute.MustGetDepValue(deps, kube, "runtime")

			if err := PrepareIngressInKube(ctx, env, kube); err != nil {
				return nil, err
			}

			fmt.Fprintln(console.Stdout(ctx), "[✓] Ensure Kubernetes ingress controller is deployed.")

			// The ingress produces no unique configuration.
			return nil, nil
		})
}

func PrepareIngressInKube(ctx context.Context, env cfg.Context, kube *kubernetes.Cluster) error {
	for _, lbl := range kube.PreparedClient().Configuration.Labels {
		if lbl.Name == ingress.LblNameStatus {
			if lbl.Value == "installed" {
				return nil
			}
			break
		}
	}

	ingressDefs, err := ingress.EnsureStack(ctx)
	if err != nil {
		return err
	}

	g := execution.NewPlan(ingressDefs...)

	// Don't wait for the deployment to complete.
	if err := execution.RawExecute(ctx, "ingress.deploy", g, execution.FromContext(env), runtime.ClusterInjection.With(kube)); err != nil {
		return err
	}

	return nil
}
