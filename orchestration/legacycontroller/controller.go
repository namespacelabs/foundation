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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
)

func Prepare(ctx context.Context, _ ExtensionDeps) error {
	config, err := rest.InClusterConfig()
	if err != nil {
		return fmt.Errorf("failed to create incluster config: %w", err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create incluster clientset: %w", err)
	}

	w := watcher{
		clientset: clientset,
	}

	w.Add(controlEphemeral, metav1.ListOptions{
		LabelSelector: kubedef.SerializeSelector(
			kubedef.SelectEphemeral(),
		),
	})
	w.Add(cleanupRuntimeConfig, metav1.ListOptions{
		LabelSelector: kubedef.SerializeSelector(
			kubedef.ManagedByUs(),
		),
	})

	// TODO remodel dev controller (removal of unused deps) with incluster-NS
	// w.Add(controlDev, metav1.ListOptions{
	// 	LabelSelector: kubedef.SerializeSelector(
	// 		kubedef.SelectByPurpose(schema.Environment_DEVELOPMENT),
	// 	),
	// })

	w.Run(context.Background())

	return nil
}

type controllerFunc func(context.Context, *kubernetes.Clientset, *corev1.Namespace, chan struct{})

type controller struct {
	opts    metav1.ListOptions
	tracked map[string]chan struct{}
	f       controllerFunc
}

type watcher struct {
	clientset   *kubernetes.Clientset
	controllers []controller
}

func (w *watcher) Add(f controllerFunc, opts metav1.ListOptions) {
	w.controllers = append(w.controllers, controller{
		opts:    opts,
		tracked: make(map[string]chan struct{}),
		f:       f,
	})
}

func (w *watcher) Run(ctx context.Context) {
	for _, controller := range w.controllers {
		go watchNamespaces(ctx, w.clientset, controller)
	}
}

func watchNamespaces(ctx context.Context, clientset *kubernetes.Clientset, c controller) {
	w, err := clientset.CoreV1().Namespaces().Watch(ctx, c.opts)
	if err != nil {
		// This is a critical failure, so log.Fatalf could be justified.
		// However, the legacy controller is best-effort & we will remodel it soon, so let's not kill the orchestrator here.
		fmt.Fprintf(os.Stderr, "failed to watch namespaces: %v", err)
		return
	}

	defer w.Stop()

	for {
		ev, ok := <-w.ResultChan()
		if !ok {
			log.Printf("namespace watch closure - retrying")
			go watchNamespaces(ctx, clientset, c)
			return
		}
		ns, ok := ev.Object.(*corev1.Namespace)
		if !ok {
			log.Printf("received non-namespace watch event: %v\n", reflect.TypeOf(ev.Object))
			continue
		}

		if done, ok := c.tracked[ns.Name]; ok {
			if ns.Status.Phase == corev1.NamespaceTerminating {
				log.Printf("Stopping watch on %q. It is terminating.", ns.Name)
				done <- struct{}{}

				delete(c.tracked, ns.Name)
			}
			continue
		}

		if ns.Status.Phase == corev1.NamespaceTerminating {
			continue
		}

		done := make(chan struct{})
		c.tracked[ns.Name] = done

		go c.f(ctx, clientset, ns, done)
	}
}
