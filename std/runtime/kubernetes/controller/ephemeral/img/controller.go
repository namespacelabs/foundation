// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"context"
	"log"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
)

const (
	interval  = time.Minute
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

	for {
		// TODO consider using .Watch(...)
		list, err := clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{
			LabelSelector: kubedef.SerializeSelector(
				kubedef.SelectEphemeral(),
			),
		})
		if err != nil {
			log.Fatalf("failed to list namespaces: %v", err)
		}

		log.Printf("found %d ephemeral namespaces", len(list.Items))

		for _, ns := range list.Items {
			if ns.Status.Phase == corev1.NamespaceTerminating {
				log.Printf("Skipping namespace %q. It is already terminating.", ns.Name)
				continue
			}

			// TODO consider using .Watch(...)
			events, err := clientset.CoreV1().Events(ns.Name).List(ctx, metav1.ListOptions{})
			if err != nil {
				log.Fatalf("failed to list events in namespace %q: %v", ns.Name, err)
			}
			if len(events.Items) == 0 {
				log.Printf("No events found for namespace %q. Skipping for now.", ns.Name)
				// TODO what if a namespace never has events?
				continue
			}
			lastTimestamp := events.Items[0].LastTimestamp

			for _, e := range events.Items {
				if lastTimestamp.Before(&e.LastTimestamp) {
					lastTimestamp = e.LastTimestamp
				}
			}

			elapsed := time.Now().Sub(lastTimestamp.Time)
			if elapsed > killAfter {
				log.Printf("Deleting stale ephemeral namespace %s. Saw no new event since %s", ns.Name, elapsed)
				if err := clientset.CoreV1().Namespaces().Delete(ctx, ns.Name, metav1.DeleteOptions{}); err != nil {
					log.Fatalf("failed to delete namespace %s: %v", ns.Name, err)
				}
			} else {
				log.Printf("Last event for namespace %q was %s ago. Let's not delete it yet.", ns.Name, elapsed)
			}
		}

		log.Printf("Will check again in %s.", interval)
		time.Sleep(interval)
	}
}
