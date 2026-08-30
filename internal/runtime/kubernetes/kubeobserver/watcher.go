// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package kubeobserver

import (
	"context"
	"fmt"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s "k8s.io/client-go/kubernetes"
	"namespacelabs.dev/foundation/framework/kubernetes/kubeobj"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
)

func WatchPods[V any](ctx context.Context, cli *k8s.Clientset, namespace string, labels map[string]string, callback func(corev1.Pod) (V, bool, error)) (V, error) {
	var empty V

	for {
		w, err := cli.CoreV1().Pods(namespace).Watch(ctx, metav1.ListOptions{LabelSelector: kubeobj.SerializeSelector(labels)})
		if err != nil {
			return empty, fnerrors.InvocationError("kubernetes", "failed while waiting for pod: %w", err)
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

			v, done, err := callback(*pod)
			if err != nil {
				return v, err
			}

			if done {
				return v, nil
			}
		}
	}
}
