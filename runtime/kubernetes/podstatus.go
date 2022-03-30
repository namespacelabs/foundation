// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubernetes

import (
	"context"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s "k8s.io/client-go/kubernetes"
	"namespacelabs.dev/foundation/internal/uniquestrings"
)

func podWaitingStatus(ctx context.Context, cli *k8s.Clientset, ns string, replicaset string) (string, error) {
	pods, err := cli.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return "", err
	}

	var reasons uniquestrings.List
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

		for _, s := range pod.Status.ContainerStatuses {
			if s.State.Waiting != nil && s.State.Waiting.Reason != "" {
				reasons.Add(s.State.Waiting.Reason)
			}
		}
	}

	return strings.Join(reasons.Strings(), ","), nil
}
