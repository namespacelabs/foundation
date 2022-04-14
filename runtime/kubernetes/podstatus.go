// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubernetes

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s "k8s.io/client-go/kubernetes"
	"namespacelabs.dev/foundation/internal/engine/ops"
)

func podWaitingStatus(ctx context.Context, cli *k8s.Clientset, ns string, replicaset string) ([]ops.WaitStatus, error) {
	// TODO explore how to limit the list here (e.g. through labels or by using a different API)
	pods, err := cli.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var statuses []ops.WaitStatus
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

		statuses = append(statuses, waiterFromPodStatus(pod.Namespace, pod.Name, pod.Status))
	}

	return statuses, nil
}
