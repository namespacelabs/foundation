// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package ingress

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"namespacelabs.dev/foundation/framework/kubernetes/kubedef"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes/client"
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

		if z.ReadinessServiceName != "" {
			if err := tasks.Action("ingress.wait-service").HumanReadable("Waiting until Ingress controller service is ready").Run(ctx, func(ctx context.Context) error {
				return waitForServiceEndpoints(ctx, kube.PreparedClient().Clientset, z.InClusterController.GetNamespace(), z.ReadinessServiceName)
			}); err != nil {
				return nil, err
			}
		}

		return ingress, nil
	})
}

func waitForServiceEndpoints(ctx context.Context, cli kubernetes.Interface, namespace, name string) error {
	// Deployment readiness and endpoint publication are separate controller
	// updates. Wait for the latter before allowing fail-closed admission webhooks
	// to receive Ingress resources.
	err := client.PollImmediateWithContext(ctx, 500*time.Millisecond, 5*time.Minute, func(ctx context.Context) (bool, error) {
		endpoints, err := cli.CoreV1().Endpoints(namespace).Get(ctx, name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		if err != nil {
			return false, err
		}

		return hasReadyEndpoint(endpoints), nil
	})
	if err != nil {
		return fmt.Errorf("waiting for service %s/%s to have ready endpoints: %w", namespace, name, err)
	}
	return nil
}

func hasReadyEndpoint(endpoints *corev1.Endpoints) bool {
	for _, subset := range endpoints.Subsets {
		if len(subset.Addresses) > 0 {
			return true
		}
	}
	return false
}

func EnsureState(ctx context.Context, cluster kubedef.KubeCluster, ingressClass string) (kubedef.IngressClass, error) {
	ingress, err := cluster.EnsureKeyedState(ctx, ingressStateKey, ingressClass)
	if err != nil {
		return nil, err
	}
	return ingress.(kubedef.IngressClass), nil
}
