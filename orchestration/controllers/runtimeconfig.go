// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package controllers

import (
	"context"
	"fmt"
	"log"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const minConfigLifetime = time.Minute // Don't touch freshly created runtime configs.

type RuntimeConfigReconciler struct {
	client.Client
}

func (r RuntimeConfigReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	configs := &corev1.ConfigMapList{}
	if err := r.List(ctx, configs, client.InNamespace(req.Namespace), client.MatchingLabels{kubedef.K8sKind: kubedef.K8sRuntimeConfigKind}); err != nil {
		return reconcile.Result{}, fmt.Errorf("unable to list configmaps in namespace %s: %w", req.Namespace, err)
	}

	if len(configs.Items) == 0 {
		// No runtime configs present in this namespace
		return reconcile.Result{}, nil
	}

	managedByUs := client.MatchingLabels{kubedef.AppKubernetesIoManagedBy: kubedef.ManagerId}

	usedConfigs := map[string]struct{}{}

	pods := &corev1.PodList{}
	if err := r.List(ctx, pods, client.InNamespace(req.Namespace), managedByUs); err != nil {
		return reconcile.Result{}, fmt.Errorf("unable to list pods in namespace %s: %w", req.Namespace, err)
	}
	for _, d := range pods.Items {
		if v, ok := d.Annotations[kubedef.K8sRuntimeConfig]; ok {
			usedConfigs[v] = struct{}{}
		}
	}

	// Also check deployments/stateful sets for what version will be in use next
	deployments := &appsv1.DeploymentList{}
	if err := r.List(ctx, deployments, client.InNamespace(req.Namespace), managedByUs); err != nil {
		return reconcile.Result{}, fmt.Errorf("unable to list deployments in namespace %s: %w", req.Namespace, err)
	}
	for _, d := range deployments.Items {
		if v, ok := d.Annotations[kubedef.K8sRuntimeConfig]; ok {
			usedConfigs[v] = struct{}{}
		}
	}

	statefulSets := &appsv1.StatefulSetList{}
	if err := r.List(ctx, statefulSets, client.InNamespace(req.Namespace), managedByUs); err != nil {
		return reconcile.Result{}, fmt.Errorf("unable to list stateful sets in namespace %s: %w", req.Namespace, err)
	}
	for _, d := range statefulSets.Items {
		if v, ok := d.Annotations[kubedef.K8sRuntimeConfig]; ok {
			usedConfigs[v] = struct{}{}
		}
	}

	for _, cfg := range configs.Items {
		if _, ok := usedConfigs[cfg.Name]; ok {
			continue
		}

		if time.Since(cfg.CreationTimestamp.Time) < minConfigLifetime {
			// Bail out for recently created configs to mitigate races (e.g. cfg created but deployment not applied yet).
			continue
		}

		if err := r.Delete(ctx, &cfg); err != nil {
			if k8serrors.IsNotFound(err) {
				// already deleted
				continue
			}
			log.Printf("kubernetes: failed to remove unused runtime configuration %q: %v\n", cfg.Name, err)
		}
	}

	return reconcile.Result{}, nil
}
