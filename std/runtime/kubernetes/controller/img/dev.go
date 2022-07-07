// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"context"
	"fmt"
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

			_, ok = ev.Object.(*appsv1.Deployment)
			if !ok {
				log.Printf("received non-deployment watch event for namespace %q: %v", ns.Name, reflect.TypeOf(ev.Object))
				continue
			}

			// We fetch all focus servers here to ensure we respect old deployments, too.
			l, err := clientset.AppsV1().Deployments(ns.Name).List(ctx, opts)
			if err != nil {
				log.Fatalf("failed to list focus deployments in namespace %q: %v", ns.Name, err)
			}

			if !allReady(l) {
				continue
			}

			required, err := requiredDeps(l)
			if err != nil {
				log.Printf("Unable to compute required deps: %v", err)
				continue
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

func allReady(l *appsv1.DeploymentList) bool {
	for _, focus := range l.Items {
		if focus.Status.Replicas < focus.Status.ReadyReplicas || focus.Status.Replicas < 1 {
			// Not ready yet.
			return false
		}
	}

	return true
}

func requiredDeps(l *appsv1.DeploymentList) (map[string]struct{}, error) {
	required := make(map[string]struct{})

	for _, focus := range l.Items {
		// Each focus server is required
		id, ok := focus.Labels[kubedef.K8sServerId]
		if !ok {
			return nil, fmt.Errorf("focus deployment %q in namespace %q does not have a server id", focus.Name, focus.Namespace)
		}
		required[id] = struct{}{}

		// Each stack dep of focus servers is required
		stack, ok := focus.Annotations[kubedef.K8sFocusStack]
		if !ok {
			return nil, fmt.Errorf("focus deployment %q in namespace %q does not contain a deps annotation", focus.Name, focus.Namespace)
		}

		for _, dep := range strings.Split(stack, ",") {
			required[dep] = struct{}{}
		}
	}

	return required, nil
}
