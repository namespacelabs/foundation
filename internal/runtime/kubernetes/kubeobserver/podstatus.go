// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package kubeobserver

import (
	"context"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s "k8s.io/client-go/kubernetes"
	"namespacelabs.dev/foundation/framework/kubernetes/kubedef"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/protos"
	"namespacelabs.dev/foundation/schema/orchestration"
	"namespacelabs.dev/foundation/schema/runtime"
)

func PodStatusToWaitStatus(ns, name string, ps v1.PodStatus) *orchestration.Event_WaitStatus {
	if ps.Phase == v1.PodPending && len(ps.ContainerStatuses) == 0 {
		return &orchestration.Event_WaitStatus{Description: "Pending..."}
	}

	cw := &runtime.ContainerWaitStatus{
		IsReady: matchPodCondition(ps, v1.PodReady),
	}

	for _, container := range ps.ContainerStatuses {
		if status := StatusToDiagnostic(container); status != nil {
			cw.Containers = append(cw.Containers, &runtime.ContainerUnitWaitStatus{
				Reference: kubedef.MakePodRef(ns, name, container.Name, nil),
				Name:      container.Name,
				Status:    status,
			})
		}
	}

	for _, init := range ps.InitContainerStatuses {
		if status := StatusToDiagnostic(init); status != nil {
			cw.Initializers = append(cw.Initializers, &runtime.ContainerUnitWaitStatus{
				Reference: kubedef.MakePodRef(ns, name, init.Name, nil),
				Name:      init.Name,
				Status:    status,
			})
		}
	}

	return &orchestration.Event_WaitStatus{
		Description: cw.WaitStatus(),
		Opaque:      protos.WrapAnyOrDie(cw),
	}
}

func podWaitingStatus(ctx context.Context, cli *k8s.Clientset, namespace string, replicaset string) ([]*orchestration.Event_WaitStatus, error) {
	// TODO explore how to limit the list here (e.g. through labels or by using a different API)
	pods, err := cli.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: kubedef.SerializeSelector(kubedef.ManagedByUs())})
	if err != nil {
		return nil, fnerrors.InvocationError("kubernetes", "unable to list pods: %w", err)
	}

	var statuses []*orchestration.Event_WaitStatus
	for _, pod := range pods.Items {
		owned := false
		for _, owner := range pod.ObjectMeta.OwnerReferences {
			if owner.Name == replicaset {
				owned = true
			}
		}
		if !owned {
			continue
		}

		statuses = append(statuses, PodStatusToWaitStatus(pod.Namespace, pod.Name, pod.Status))
	}

	return statuses, nil
}
