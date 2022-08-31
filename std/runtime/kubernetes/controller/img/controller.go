// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"context"
	"log"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
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

	w := watcher{
		clientset:   clientset,
		controllers: make(map[metav1.ListOptions]controllerFunc),
	}

	w.Add(controlEphemeral, metav1.ListOptions{
		LabelSelector: kubedef.SerializeSelector(
			kubedef.SelectEphemeral(),
		),
	})

	// TODO remodel dev controller (removal of unused deps) with incluster-NS
	// w.Add(controlDev, metav1.ListOptions{
	// 	LabelSelector: kubedef.SerializeSelector(
	// 		kubedef.SelectByPurpose(schema.Environment_DEVELOPMENT),
	// 	),
	// })

	w.Run(context.Background())
}

type controllerFunc func(context.Context, *kubernetes.Clientset, *corev1.Namespace, chan struct{})

type watcher struct {
	clientset   *kubernetes.Clientset
	controllers map[metav1.ListOptions]controllerFunc
}

func (w watcher) Add(c controllerFunc, opts metav1.ListOptions) {
	w.controllers[opts] = c
}

func (w watcher) Run(ctx context.Context) {
	for opts, controller := range w.controllers {
		go watchNamespaces(ctx, w.clientset, opts, controller)
	}

	// Only stop namespace watching on fatal errors
	select {}
}

func watchNamespaces(ctx context.Context, clientset *kubernetes.Clientset, opts metav1.ListOptions, f controllerFunc) {
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
				log.Printf("Stopping watch on %q. It is terminating.", ns.Name)
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

		go f(ctx, clientset, ns, done)
	}
}
