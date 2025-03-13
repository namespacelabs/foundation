// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package ingress

import (
	"context"
	"fmt"

	"namespacelabs.dev/foundation/framework/kubernetes/kubedef"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes/kubeobserver"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/tasks"
)

const ingressStateKey = "kubernetes.ingress-state"

func RegisterRuntimeState() {
	runtime.RegisterKeyedPrepare(ingressStateKey, func(ctx context.Context, cfg cfg.Configuration, cluster runtime.Cluster, ingressClass string) (any, error) {
		kube, ok := cluster.(kubedef.KubeCluster)
		if !ok {
			return nil, fnerrors.InternalError("%s: only supported with Kubernetes clusters", ingressStateKey)
		}

		ingress, err := Class(ingressClass)
		if err != nil {
			return nil, err
		}

		z := ingress.Service()
		if z == nil || z.InClusterController == nil {
			return ingress, nil
		}

		w := kubeobserver.WaitOnResource{
			RestConfig:       kube.PreparedClient().RESTConfig,
			Description:      fmt.Sprintf("Ingress Controller (%s)", ingress.Name()),
			Namespace:        z.InClusterController.GetNamespace(),
			Name:             z.InClusterController.GetName(),
			GroupVersionKind: z.InClusterController.GroupVersionKind(),
			Scope:            "namespacelabs.dev/foundation/internal/networking/ingress",
		}

		if err := tasks.Action("ingress.wait").HumanReadable("Waiting until Ingress controller is ready").Run(ctx, func(ctx context.Context) error {
			return w.WaitUntilReady(ctx, nil)
		}); err != nil {
			return nil, err
		}

		return ingress, nil
	})
}

func EnsureState(ctx context.Context, cluster kubedef.KubeCluster, ingressClass string) (kubedef.IngressClass, error) {
	ingress, err := cluster.EnsureKeyedState(ctx, ingressStateKey, ingressClass)
	if err != nil {
		return nil, err
	}
	return ingress.(kubedef.IngressClass), nil
}
