// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"context"
	"log"
	"time"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
)

func controlEphemeral(ctx context.Context, clientset *kubernetes.Clientset, ns *corev1.Namespace, done chan struct{}) {
	timeout := time.Hour

	if anno, ok := ns.Annotations[kubedef.K8sEnvTimeout]; ok {
		var err error
		if timeout, err = time.ParseDuration(anno); err != nil {
			log.Fatalf("invalid timeout annotation %q for namespace %q: %v", anno, ns.Name, err)
		}
	}

	// TODO watch test driver for test environments.

	select {
	case <-done:
		return
	case <-time.After(timeout):
		log.Printf("deleting stale ephemeral namespace %q", ns.Name)
		if err := clientset.CoreV1().Namespaces().Delete(ctx, ns.Name, metav1.DeleteOptions{}); err != nil {
			if !k8serrors.IsNotFound(err) {
				// Namespace already deleted
				return
			}
			log.Fatalf("failed to delete namespace %s: %v", ns.Name, err)
		}
	}
}
