// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubernetes

import (
	"context"
	"fmt"
	"io"

	"golang.org/x/exp/slices"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnerrors/multierr"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func (r ClusterNamespace) DeleteRecursively(ctx context.Context, wait bool) (bool, error) {
	return DeleteAllRecursively(ctx, r.cli, wait, nil, r.namespace)
}

func (r *Cluster) DeleteAllRecursively(ctx context.Context, wait bool, progress io.Writer) (bool, error) {
	namespaces, err := r.cli.CoreV1().Namespaces().List(ctx, metav1.ListOptions{
		LabelSelector: kubedef.SerializeSelector(kubedef.ManagedByUs()),
	})
	if err != nil {
		return false, fnerrors.Wrapf(nil, err, "unable to list namespaces")
	}

	var filtered []string
	for _, ns := range namespaces.Items {
		// Only delete namespaces that were used to deploy an environment.
		if _, ok := ns.Labels[kubedef.K8sEnvName]; !ok {
			continue
		}

		filtered = append(filtered, ns.Name)
	}

	return DeleteAllRecursively(ctx, r.cli, wait, progress, filtered...)
}

func DeleteAllRecursively(ctx context.Context, cli *kubernetes.Clientset, wait bool, progress io.Writer, namespaces ...string) (bool, error) {
	if len(namespaces) == 0 {
		return false, nil
	}

	return tasks.Return(ctx, tasks.Action("kubernetes.namespace.delete").Arg("namespaces", namespaces), func(ctx context.Context) (bool, error) {
		var w watch.Interface

		if wait {
			// Start watching before emitting any Delete, to make sure we observe all events.
			watcher, err := cli.CoreV1().Namespaces().Watch(ctx, metav1.ListOptions{})
			if err != nil {
				return false, err
			}

			w = watcher
			defer watcher.Stop()
		}

		var removed []string
		var errs []error
		for _, ns := range namespaces {
			var grace int64 = 0
			if err := cli.CoreV1().Namespaces().Delete(ctx, ns, metav1.DeleteOptions{
				GracePeriodSeconds: &grace,
			}); err != nil {
				// Namespace already deleted?
				if !k8serrors.IsNotFound(err) {
					errs = append(errs, err)
				}
			} else {
				removed = append(removed, ns)
			}
		}

		if len(errs) > 0 {
			return false, multierr.New(errs...)
		}

		if !wait || len(removed) == 0 {
			return len(removed) > 0, nil
		}

		return tasks.Return(ctx, tasks.Action("kubernetes.namespace.delete-wait").Arg("namespaces", removed), func(context.Context) (bool, error) {
			for ev := range w.ResultChan() {
				if ev.Type != watch.Deleted {
					continue
				}

				ns, ok := ev.Object.(*v1.Namespace)
				if !ok {
					continue
				}

				idx := slices.Index(removed, ns.Name)
				if idx >= 0 {
					removed = slices.Delete(removed, idx, idx+1)

					if progress != nil {
						fmt.Fprintf(progress, "Namespace %q removed.\n", ns.Name)
					}
				}

				if len(removed) == 0 {
					break
				}
			}

			return true, nil
		})
	})
}
