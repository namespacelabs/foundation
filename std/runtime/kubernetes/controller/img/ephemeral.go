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
)

const (
	ephemeralNsTimeout = 5 * time.Minute
)

func controlEphemeral(ctx context.Context, clientset *kubernetes.Clientset, ns *corev1.Namespace, done chan struct{}) {
	w, err := clientset.CoreV1().Events(ns.Name).Watch(ctx, metav1.ListOptions{})
	if err != nil {
		log.Fatalf("failed to watch events in namespace %q: %v", ns.Name, err)
	}

	defer w.Stop()

	lastTimestamp := time.Now()

	for {
		timeSinceEvent := time.Since(lastTimestamp)
		remaining := ephemeralNsTimeout - timeSinceEvent

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

			if lastTimestamp.After(event.LastTimestamp.Time) {
				continue
			}

			lastTimestamp = event.LastTimestamp.Time
			log.Printf("received recent event for namespace %q", ns.Name)
		}
	}
}
