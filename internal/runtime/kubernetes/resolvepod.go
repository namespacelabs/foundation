// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubernetes

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"namespacelabs.dev/foundation/internal/fnerrors"
)

func resolvePodByLabels(ctx context.Context, cli *kubernetes.Clientset, w io.Writer, ns string, labels map[string]string) (corev1.Pod, error) {
	var kvs []string
	for k, v := range labels {
		kvs = append(kvs, fmt.Sprintf("%s=%s", k, v))
	}
	sort.Strings(kvs)
	labelSel := strings.Join(kvs, ",")

	for k := 0; ; k++ {
		if k > 0 {
			fmt.Fprintln(w, "Resolving pods...")
		}

		pods, err := cli.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{LabelSelector: labelSel})
		if err != nil {
			return corev1.Pod{}, fnerrors.Wrapf(nil, err, "unable to list pods")
		}

		if len(pods.Items) == 0 {
			fmt.Fprintln(w, "  (no pod returned)")
		} else if len(pods.Items) > 1 {
			for _, pod := range pods.Items {
				fmt.Fprintf(w, "  pod: %s (%s, started: %v)\n", pod.Name, pod.Status.Phase, pod.Status.StartTime)
			}
		}

		// If there are pods starting, wait until they resolve.
		var running, pending []corev1.Pod
		for _, pod := range pods.Items {
			switch pod.Status.Phase {
			case corev1.PodPending:
				pending = append(pending, pod)
			case corev1.PodRunning:
				running = append(running, pod)
			}
		}

		if len(pending) == 0 {
			var withStart []corev1.Pod
			for _, pod := range running {
				if pod.Status.StartTime != nil {
					withStart = append(withStart, pod)
				}
			}

			if len(withStart) > 0 {
				sort.SliceStable(withStart, func(i, j int) bool {
					a, b := running[i].Status, running[j].Status
					return !a.StartTime.Before(b.StartTime) // Sort the newly created pods first.
				})

				return withStart[0], nil
			}
		}

		fmt.Fprintln(w, "Waiting one second...")
		time.Sleep(1 * time.Second)
	}
}
