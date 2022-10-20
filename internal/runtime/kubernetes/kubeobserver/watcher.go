// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubeobserver

import (
	"context"
	"fmt"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s "k8s.io/client-go/kubernetes"
	"namespacelabs.dev/foundation/framework/kubernetes/kubedef"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/std/tasks"
)

func WatchDeployable[V any](ctx context.Context, actionName string, cli *k8s.Clientset, namespace string, object runtime.Deployable, callback func(corev1.Pod) (V, bool)) (V, error) {
	return tasks.Return(ctx, tasks.Action(actionName).Arg("id", object.GetId()).Arg("name", object.GetName()),
		func(ctx context.Context) (V, error) {
			return WatchPods(ctx, cli, namespace, kubedef.SelectById(object), callback)
		})
}

func WatchPods[V any](ctx context.Context, cli *k8s.Clientset, namespace string, labels map[string]string, callback func(corev1.Pod) (V, bool)) (V, error) {
	var empty V

	for {
		w, err := cli.CoreV1().Pods(namespace).Watch(ctx, metav1.ListOptions{LabelSelector: kubedef.SerializeSelector(labels)})
		if err != nil {
			return empty, fnerrors.InternalError("kubernetes: failed while waiting for pod: %w", err)
		}

		defer w.Stop()

		debug := console.Debug(ctx)

		for ev := range w.ResultChan() {
			if ev.Object == nil {
				continue
			}

			pod, ok := ev.Object.(*corev1.Pod)
			if !ok {
				fmt.Fprintf(debug, "received non-pod event: %v\n", reflect.TypeOf(ev.Object))
				continue
			}

			v, done := callback(*pod)
			if done {
				return v, nil
			}
		}
	}
}
