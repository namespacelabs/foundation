// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"context"
	"log"
	"reflect"
	"time"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
)

const (
	killAfter = 5 * time.Minute
)

func controlEphemeral(ctx context.Context, clientset *kubernetes.Clientset) {
	opts := metav1.ListOptions{
		LabelSelector: kubedef.SerializeSelector(
			kubedef.SelectEphemeral(),
		),
	}

	w, err := clientset.CoreV1().Namespaces().Watch(ctx, opts)
	if err != nil {
		log.Fatalf("failed to watch namespaces: %v", err)
	}

	defer w.Stop()

	tracked := make(map[string]chan struct{})
	for {
		ev, ok := <-w.ResultChan()
		if !ok {
			log.Fatalf("unexpected namespace watch closure: %v", err)
		}
		ns, ok := ev.Object.(*corev1.Namespace)
		if !ok {
			log.Printf("received non-namespace watch event: %v\n", reflect.TypeOf(ev.Object))
			continue
		}

		if done, ok := tracked[ns.Name]; ok {
			if ns.Status.Phase == corev1.NamespaceTerminating {
				log.Printf("Stopping watch on %q. It is already terminating.", ns.Name)
				done <- struct{}{}

				delete(tracked, ns.Name)
			}
			continue
		}

		if ns.Status.Phase == corev1.NamespaceTerminating {
			continue
		}

		done := make(chan struct{})
		tracked[ns.Name] = done
		log.Printf("Starting watch on ephemeral namespace %q", ns.Name)
		go watchNamespace(ctx, clientset, ns, done)

		log.Printf("Watching %d ephemeral namespaces.", len(tracked))
	}
}

func watchNamespace(ctx context.Context, clientset *kubernetes.Clientset, ns *corev1.Namespace, done chan struct{}) {
	w, err := clientset.CoreV1().Events(ns.Name).Watch(ctx, metav1.ListOptions{})
	if err != nil {
		log.Fatalf("failed to watch events in namespace %q: %v", ns.Name, err)
	}

	defer w.Stop()

	lastTimestamp := time.Now()

	for {
		remaining := killAfter - time.Since(lastTimestamp)

		select {
		case <-done:
			return
		case <-time.After(remaining):
			log.Printf("deleting stale ephemeral namespace %q", ns.Name)
			if err := clientset.CoreV1().Namespaces().Delete(ctx, ns.Name, metav1.DeleteOptions{}); err != nil {
				if !k8serrors.IsNotFound(err) {
					// Namespace already deleted
					return
				}
				log.Fatalf("failed to delete namespace %s: %v", ns.Name, err)
			}
			return

		case ev, ok := <-w.ResultChan():
			if !ok {
				log.Fatalf("unexpected event watch closure for namespace %q: %v", ns.Name, err)
			}

			event, ok := ev.Object.(*corev1.Event)
			if !ok {
				log.Printf("received non-event watch event for namespace %q: %v", ns.Name, reflect.TypeOf(ev.Object))
				continue
			}

			if lastTimestamp.Before(event.LastTimestamp.Time) {
				lastTimestamp = event.LastTimestamp.Time
				log.Printf("received recent event for namespace %q", ns.Name)
			}
		}
	}
}
