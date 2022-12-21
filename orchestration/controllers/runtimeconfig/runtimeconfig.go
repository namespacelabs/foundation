// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package runtimeconfig

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	"namespacelabs.dev/foundation/framework/kubernetes/kubedef"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	// Don't touch freshly created runtime configs.
	// We reconcile on pod updates. If there is an independent pod update after a new runtime config was created,
	// but before it is referenced by a consumer, we must not delete the new config.
	// In that case, we requeue when the config is old enough and check if references appeared meanwhile.
	minConfigLifetime   = 15 * time.Minute
	DeleteRuntimeConfig = "DeleteRuntimeConfig"

	K8sObjectObsolete = "k8s.namespacelabs.dev/object-obsolete"
)

type RuntimeConfigReconciler struct {
	client client.Client
}

func (r *RuntimeConfigReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	configs := &corev1.ConfigMapList{}
	if err := r.client.List(ctx, configs, client.InNamespace(req.Namespace), client.MatchingLabels{kubedef.K8sKind: kubedef.K8sRuntimeConfigKind}); err != nil {
		return reconcile.Result{}, fmt.Errorf("unable to list configmaps in namespace %s: %w", req.Namespace, err)
	}

	if len(configs.Items) == 0 {
		// No runtime configs present in this namespace
		return reconcile.Result{}, nil
	}

	managedByUs := client.MatchingLabels{kubedef.AppKubernetesIoManagedBy: kubedef.ManagerId}

	usedConfigs := map[string]struct{}{}

	pods := &corev1.PodList{}
	if err := r.client.List(ctx, pods, client.InNamespace(req.Namespace), managedByUs); err != nil {
		return reconcile.Result{}, fmt.Errorf("unable to list pods in namespace %s: %w", req.Namespace, err)
	}
	for _, d := range pods.Items {
		if v, ok := d.Annotations[kubedef.K8sRuntimeConfig]; ok {
			usedConfigs[v] = struct{}{}
		}
	}

	// Also check deployments/stateful sets for what version will be in use next
	deployments := &appsv1.DeploymentList{}
	if err := r.client.List(ctx, deployments, client.InNamespace(req.Namespace), managedByUs); err != nil {
		return reconcile.Result{}, fmt.Errorf("unable to list deployments in namespace %s: %w", req.Namespace, err)
	}
	for _, d := range deployments.Items {
		if v, ok := d.Annotations[kubedef.K8sRuntimeConfig]; ok {
			usedConfigs[v] = struct{}{}
		}
	}

	statefulSets := &appsv1.StatefulSetList{}
	if err := r.client.List(ctx, statefulSets, client.InNamespace(req.Namespace), managedByUs); err != nil {
		return reconcile.Result{}, fmt.Errorf("unable to list stateful sets in namespace %s: %w", req.Namespace, err)
	}
	for _, d := range statefulSets.Items {
		if v, ok := d.Annotations[kubedef.K8sRuntimeConfig]; ok {
			usedConfigs[v] = struct{}{}
		}
	}

	for _, cfg := range configs.Items {
		_, used := usedConfigs[cfg.Name]
		cfg.Labels[K8sObjectObsolete] = fmt.Sprintf("%t", !used)

		if err := r.client.Update(ctx, &cfg); err != nil {
			return reconcile.Result{}, fmt.Errorf("unable to update config map %s in namespace %s: %w", cfg.Name, cfg.Namespace, err)
		}
	}

	return reconcile.Result{}, nil
}

type RuntimeConfigGC struct {
	client   client.Client
	recorder record.EventRecorder
}

func (r *RuntimeConfigGC) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	configs := &corev1.ConfigMapList{}
	if err := r.client.List(ctx, configs, client.InNamespace(req.Namespace), client.MatchingLabels{
		kubedef.K8sKind:   kubedef.K8sRuntimeConfigKind,
		K8sObjectObsolete: "true",
	}); err != nil {
		return reconcile.Result{}, fmt.Errorf("unable to list configmaps in namespace %s: %w", req.Namespace, err)
	}

	requeueAfter := minConfigLifetime
	for _, cfg := range configs.Items {
		lifetime := time.Since(cfg.CreationTimestamp.Time)
		if lifetime < minConfigLifetime {
			deleteAfter := minConfigLifetime - lifetime
			if deleteAfter < requeueAfter {
				requeueAfter = deleteAfter
			}
			continue
		}

		if err := r.client.Delete(ctx, &cfg); err != nil {
			if k8serrors.IsNotFound(err) {
				// already deleted
				continue
			}
			r.recorder.Eventf(&cfg, corev1.EventTypeWarning, DeleteRuntimeConfig, "failed to remove unused runtime configuration %q: %v", cfg.Name, err)
		}
	}

	if requeueAfter < minConfigLifetime {
		return reconcile.Result{RequeueAfter: requeueAfter}, nil
	}

	return reconcile.Result{}, nil
}
