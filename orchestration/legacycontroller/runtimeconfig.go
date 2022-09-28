// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package legacycontroller

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
)

const minConfigLifetime = time.Minute // Don't touch freshly created runtime configs.

func cleanupRuntimeConfig(ctx context.Context, clientset *kubernetes.Clientset, ns *corev1.Namespace, done chan struct{}) {
	w, err := clientset.CoreV1().Pods(ns.Name).Watch(ctx, metav1.ListOptions{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "skipping namespace %q: failed to watch pods: %v", ns.Name, err)
		return
	}

	for {
		select {
		case <-done:
			// Namespace is terminating
			return
		case _, ok := <-w.ResultChan():
			if !ok {
				log.Printf("watch closure for namespace %q - retrying", ns.Name)
				go cleanupRuntimeConfig(ctx, clientset, ns, done)
				return
			}

			// Pods updated - check if we need to clean up stale runtime configs.
			configs, err := clientset.CoreV1().ConfigMaps(ns.Name).List(ctx, metav1.ListOptions{
				LabelSelector: kubedef.SerializeSelector(map[string]string{
					kubedef.K8sKind: kubedef.K8sRuntimeConfigKind,
				}),
			})
			if err != nil {
				log.Printf("unable to list configmaps in namespace %q", ns.Name)
				continue
			}

			if len(configs.Items) == 0 {
				// No runtime configs present in this namespace
				continue
			}

			listOpts := metav1.ListOptions{
				LabelSelector: kubedef.SerializeSelector(kubedef.ManagedByUs()),
			}

			usedConfigs := map[string]struct{}{}
			pods, err := clientset.CoreV1().Pods(ns.Name).List(ctx, listOpts)
			if err != nil {
				log.Printf("unable to list pods in namespace %q", ns.Name)
				continue
			}

			for _, d := range pods.Items {
				if v, ok := d.Annotations[kubedef.K8sRuntimeConfig]; ok {
					usedConfigs[v] = struct{}{}
				}
			}

			// Also check deployments/stateful sets for what version will be in use next
			deployments, err := clientset.AppsV1().Deployments(ns.Name).List(ctx, listOpts)
			if err != nil {
				log.Printf("unable to list deployments in namespace %q", ns.Name)
				continue
			}

			for _, d := range deployments.Items {
				if v, ok := d.Annotations[kubedef.K8sRuntimeConfig]; ok {
					usedConfigs[v] = struct{}{}
				}
			}

			statefulSets, err := clientset.AppsV1().StatefulSets(ns.Name).List(ctx, listOpts)
			if err != nil {
				log.Printf("unable to list stateful sets in namespace %q", ns.Name)
				continue
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

				if err := clientset.CoreV1().ConfigMaps(ns.Name).Delete(ctx, cfg.Name, metav1.DeleteOptions{}); err != nil {
					if k8serrors.IsNotFound(err) {
						// already deleted
						continue
					}
					log.Printf("kubernetes: failed to remove unused runtime configuration %q: %v\n", cfg.Name, err)
				}
			}
		}
	}
}
