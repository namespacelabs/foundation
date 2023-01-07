// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package prepare

import (
	"context"

	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/schema/orchestration"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/execution"
)

func Ingress() ClusterStage {
	return ClusterStage{
		Pre: func(ch chan *orchestration.Event) {
			ch <- &orchestration.Event{
				ResourceId:    "ingress",
				Scope:         "Setup Ingress Controller", // XXX remove soon.
				ResourceLabel: "Setup Ingress Controller",
				Category:      "Preparing cluster",
				Ready:         orchestration.Event_NOT_READY,
				Stage:         orchestration.Event_WAITING,
			}
		},
		Post: func(ch chan *orchestration.Event) {
			ch <- &orchestration.Event{
				ResourceId: "ingress",
				Ready:      orchestration.Event_READY,
				Stage:      orchestration.Event_DONE,
			}
		},
		Run: func(ctx context.Context, env cfg.Context, devhost *schema.DevHost_ConfigureEnvironment, kube *kubernetes.Cluster, ch chan *orchestration.Event) error {
			return PrepareIngressInKube(ctx, env, kube)
		},
	}
}

func PrepareIngressInKube(ctx context.Context, env cfg.Context, kube *kubernetes.Cluster) error {
	ingress := kube.Ingress()
	if ingress == nil {
		return nil
	}

	ingressDefs, err := ingress.Ensure(ctx)
	if err != nil {
		return err
	}

	g := execution.NewPlan(ingressDefs...)

	// Don't wait for the deployment to complete.
	if err := execution.RawExecute(ctx, "ingress.deploy", execution.ExecuteOpts{
		ContinueOnErrors: true,
	}, g, execution.FromContext(env), runtime.ClusterInjection.With(kube)); err != nil {
		return err
	}

	return nil
}
