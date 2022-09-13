// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package legacycontroller

import (
	"context"
	"fmt"
	"log"
	"os"
	"reflect"
	"time"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
)

const gracePeriod = 5 * time.Minute

func controlEphemeral(ctx context.Context, clientset *kubernetes.Clientset, ns *corev1.Namespace, done chan struct{}) {
	timeout := time.Hour

	if anno, ok := ns.Annotations[kubedef.K8sEnvTimeout]; ok {
		var err error
		if timeout, err = time.ParseDuration(anno); err != nil {
			fmt.Fprintf(os.Stderr, "skipping namespace %q: invalid timeout annotation %q: %v", ns.Name, anno, err)
			return
		}
	}

	w, err := clientset.CoreV1().Pods(ns.Name).Watch(ctx, metav1.ListOptions{
		LabelSelector: kubedef.SerializeSelector(kubedef.SelectNamespaceDriver()),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "skipping namespace %q: failed to watch driver pod: %v", ns.Name, err)
		return
	}

	for {
		remaining := time.Until(ns.CreationTimestamp.Time.Add(timeout))
		if remaining > 0 {
			log.Printf("namespace %s with timeout %s will be deleted in %s", ns.Name, timeout, remaining.Round(time.Second))
		}

		select {
		case <-done:
			return
		case <-time.After(remaining):
			if err := deleteNs(ctx, clientset, ns.Name); err != nil {
				log.Printf("failed to delete namespace %s: %v", ns.Name, err)
				return
			}
			return

		case ev, ok := <-w.ResultChan():
			if !ok {
				log.Printf("watch closure for namespace %q - retrying", ns.Name)
				go controlEphemeral(ctx, clientset, ns, done)
				return
			}

			driver, ok := ev.Object.(*corev1.Pod)
			if !ok {
				fmt.Fprintf(os.Stderr, "received non-pod watch event for namespace %q: %v", ns.Name, reflect.TypeOf(ev.Object))
				continue
			}

			if driver.Status.Phase != corev1.PodFailed && driver.Status.Phase != corev1.PodSucceeded {
				// driver not finished yet
				continue
			}

			// We add a grace period to avoid racing with log collection from a client.
			log.Printf("Driver %s finished. Namespace %s will be deleted in %s", driver.Name, ns.Name, gracePeriod)

			select {
			case <-done:
				return
			case <-time.After(gracePeriod):
				if err := deleteNs(ctx, clientset, ns.Name); err != nil {
					fmt.Fprintf(os.Stderr, "failed to delete namespace %s: %v", ns.Name, err)
				}
				return
			}
		}
	}
}

func deleteNs(ctx context.Context, clientset *kubernetes.Clientset, name string) error {
	log.Printf("deleting namespace %q", name)
	err := clientset.CoreV1().Namespaces().Delete(ctx, name, metav1.DeleteOptions{})
	if k8serrors.IsNotFound(err) {
		// Namespace already deleted
		return nil
	}

	return err
}
