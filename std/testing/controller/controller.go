// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"context"
	"log"
	"time"

	"github.com/rs/zerolog"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/schema"
)

const (
	interval  = time.Second      // TODO: time.Minute
	killAfter = 10 * time.Second // TODO
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
				kubedef.SelectByPurpose(schema.Environment_TESTING),
				kubedef.SelectEphemeral(),
			),
		})
		if err != nil {
			log.Fatalf("failed to list namespaces: %v", err)
		}

		for _, ns := range list.Items {
			// TODO remove
			zerolog.Ctx(ctx).Info().Str("status", string(ns.Status.Phase)).Msgf("Namespace=%s", ns.Name)

			if ns.Status.Phase == corev1.NamespaceTerminating {
				// TODO Add more filtering?
				continue
			}

			// TODO consider using .Watch(...)
			events, err := clientset.CoreV1().Events(ns.Name).List(ctx, metav1.ListOptions{})
			if err != nil {
				log.Fatalf("failed to list events in namespace %s: %v", ns.Name, err)
			}
			if len(events.Items) == 0 {
				continue
			}
			lastTimestamp := events.Items[0].LastTimestamp

			for _, e := range events.Items {
				if lastTimestamp.Before(&e.LastTimestamp) {
					lastTimestamp = e.LastTimestamp
				}
			}

			if time.Now().Sub(lastTimestamp.Time) > killAfter {
				log.Printf("Deleting stale testing namespace %s", ns.Name)
				if err := clientset.CoreV1().Namespaces().Delete(ctx, ns.Name, metav1.DeleteOptions{}); err != nil {
					log.Fatalf("failed to delete namespace %s: %v", ns.Name, err)
				}
			}
		}

		time.Sleep(interval)
	}
}
