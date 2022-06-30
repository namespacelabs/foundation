// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"context"
	"log"
	"reflect"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
)

func controlDev(ctx context.Context, clientset *kubernetes.Clientset, ns *corev1.Namespace, done chan struct{}) {
	opts := metav1.ListOptions{
		LabelSelector: kubedef.SerializeSelector(
			kubedef.SelectFocusServer(),
		),
	}

	l, err := clientset.AppsV1().Deployments(ns.Name).List(ctx, opts)
	if err != nil {
		log.Fatalf("failed to list focus deployments in namespace %q: %v", ns.Name, err)
	}

	// Ensure that we only consider current focus servers when cleaning up unused deps.
	opts.ResourceVersion = l.ResourceVersion

	w, err := clientset.AppsV1().Deployments(ns.Name).Watch(ctx, opts)
	if err != nil {
		log.Fatalf("failed to watch focus deployments in namespace %q: %v", ns.Name, err)
	}

	defer w.Stop()

	log.Printf("watching namespace %s for updates to focus deployment", ns.Name)

	for {
		select {
		case <-done:
			return

		case ev, ok := <-w.ResultChan():
			if !ok {
				log.Fatalf("unexpected event watch closure for namespace %q: %v", ns.Name, err)
			}

			focus, ok := ev.Object.(*appsv1.Deployment)
			if !ok {
				log.Printf("received non-deployment watch event for namespace %q: %v", ns.Name, reflect.TypeOf(ev.Object))
				continue
			}

			if focus.Status.Replicas < focus.Status.ReadyReplicas || focus.Status.Replicas < 1 {
				// Not ready yet.
				continue
			}

			stack, ok := focus.Annotations[kubedef.K8sFocusStack]
			if !ok {
				log.Printf("focus deployment %q in namespace %q does not contain a deps annotation", focus.Name, ns.Name)
				continue
			}

			log.Printf("updated focus deployment %q in namespace %q", focus.Name, ns.Name)
			required := make(map[string]struct{})
			for _, dep := range strings.Split(stack, ",") {
				required[dep] = struct{}{}
			}

			// We only clean up deployments here. Consider cleaning up other resources (e.g. stateful sets).
			list, err := clientset.AppsV1().Deployments(ns.Name).List(ctx, metav1.ListOptions{})
			if err != nil {
				log.Fatalf("failed to list deployments in namespace %q: %v", ns.Name, err)
			}

			for _, d := range list.Items {
				id, ok := d.Labels[kubedef.K8sServerId]
				if !ok {
					log.Printf("deployment %q in namespace %q does not have a server id", d.Name, ns.Name)
					continue
				}

				if _, ok := required[id]; ok {
					continue
				}

				log.Printf("deleting obsolete deployment %q in namespace %q", d.Name, ns.Name)

				if err := clientset.AppsV1().Deployments(ns.Name).Delete(ctx, d.Name, metav1.DeleteOptions{}); err != nil {
					if k8serrors.IsNotFound(err) {
						// deployment already deleted
						continue
					}
					log.Fatalf("failed to delete deployment %q in namespace %q: %v", d.Name, ns.Name, err)
				}
			}
		}
	}
}
