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
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubeobserver"
)

func (r *ClusterNamespace) ResolveContainers(ctx context.Context, object runtime.Deployable) ([]*runtime.ContainerReference, error) {
	return kubeobserver.WatchDeployable(ctx, "deployable.resolve-containers", r.cluster.cli, r.target.namespace, object, func(pod corev1.Pod) ([]*runtime.ContainerReference, bool) {
		if pod.Status.Phase != corev1.PodRunning && pod.Status.Phase != corev1.PodFailed && pod.Status.Phase != corev1.PodSucceeded {
			return nil, false
		}

		var refs []*runtime.ContainerReference

		for _, init := range pod.Status.InitContainerStatuses {
			refs = append(refs, kubedef.MakePodRef(pod.Namespace, pod.Name, init.Name, object))
		}
		for _, container := range pod.Status.ContainerStatuses {
			refs = append(refs, kubedef.MakePodRef(pod.Namespace, pod.Name, container.Name, object))
		}

		return refs, true
	})
}

func (r *ClusterNamespace) resolvePod(ctx context.Context, cli *kubernetes.Clientset, w io.Writer, obj runtime.Deployable) (corev1.Pod, error) {
	return resolvePodByLabels(ctx, cli, w, r.target.namespace, map[string]string{
		kubedef.K8sServerId: obj.GetId(),
	})
}

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
