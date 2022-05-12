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
	"k8s.io/client-go/rest"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
)

const (
	killAfter = 5 * time.Minute
)

func main() {
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalf("failed to create incluster config: %v", err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("failed to create incluster clientset: %v", err)
	}

	ctx := context.Background()

	w, err := clientset.CoreV1().Namespaces().Watch(ctx, metav1.ListOptions{
		LabelSelector: kubedef.SerializeSelector(
			kubedef.SelectEphemeral(),
		),
	})
	if err != nil {
		log.Fatalf("failed to watch namespaces: %v", err)
	}

	defer w.Stop()

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

		if ns.Status.Phase == corev1.NamespaceTerminating {
			log.Printf("Skipping namespace %q. It is already terminating.", ns.Name)
			continue
		}

		go watchNamespace(ctx, clientset, ns)
	}

}

func watchNamespace(ctx context.Context, clientset *kubernetes.Clientset, ns *corev1.Namespace) {
	w, err := clientset.CoreV1().Events(ns.Name).Watch(ctx, metav1.ListOptions{})
	if err != nil {
		log.Fatalf("failed to watch events in namespace %q: %v", ns.Name, err)
	}

	defer w.Stop()

	lastTimestamp := time.Now()

	for {
		remaining := killAfter - time.Now().Sub(lastTimestamp)

		select {
		case <-time.After(remaining):
			log.Printf("deleting stale ephemeral namespace %q", ns.Name)
			err := clientset.CoreV1().Namespaces().Delete(ctx, ns.Name, metav1.DeleteOptions{})
			if err != nil && !k8serrors.IsNotFound(err) {
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
