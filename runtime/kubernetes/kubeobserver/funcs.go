// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubeobserver

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s "k8s.io/client-go/kubernetes"
)

func PickPod(ns, name string) func(ctx context.Context, c *k8s.Clientset) ([]corev1.Pod, error) {
	return func(ctx context.Context, c *k8s.Clientset) ([]corev1.Pod, error) {
		pod, err := c.CoreV1().Pods(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}

		return []corev1.Pod{*pod}, nil
	}
}
