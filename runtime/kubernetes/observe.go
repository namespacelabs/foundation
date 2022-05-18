// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubernetes

import (
	"context"
	"fmt"
	"sort"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/schema"
)

func (r k8sRuntime) Observe(ctx context.Context, srv *schema.Server, opts runtime.ObserveOpts, onInstance func(runtime.ObserveEvent) error) error {
	// XXX use a watch
	announced := map[string]runtime.ContainerReference{}

	ns := serverNamespace(r.boundEnv, srv)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// No cancelation, moving along.
		}

		pods, err := r.cli.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{
			LabelSelector: kubedef.SerializeSelector(kubedef.SelectById(srv)),
		})
		if err != nil {
			return err
		}

		type Key struct {
			Instance  runtime.ContainerReference
			CreatedAt time.Time // used for sorting
		}
		keys := []Key{}
		newM := map[string]struct{}{}
		labels := map[string]string{}
		for _, pod := range pods.Items {
			if pod.Status.Phase == v1.PodRunning {
				instance := makePodRef(ns, pod.Name, serverCtrName(srv))
				keys = append(keys, Key{
					Instance:  instance,
					CreatedAt: pod.CreationTimestamp.Time,
				})
				newM[instance.UniqueID()] = struct{}{}
				labels[instance.UniqueID()] = srv.Name

				if ObserveInitContainerLogs {
					for _, container := range pod.Spec.InitContainers {
						instance := makePodRef(ns, pod.Name, container.Name)
						keys = append(keys, Key{Instance: instance, CreatedAt: pod.CreationTimestamp.Time})
						newM[instance.UniqueID()] = struct{}{}
						labels[instance.UniqueID()] = fmt.Sprintf("%s:%s", srv.Name, container.Name)
					}
				}
			}
		}
		sort.SliceStable(keys, func(i int, j int) bool {
			return keys[i].CreatedAt.Before(keys[j].CreatedAt)
		})

		for k, ref := range announced {
			if _, ok := newM[k]; ok {
				delete(newM, k)
			} else {
				if err := onInstance(runtime.ObserveEvent{ContainerReference: ref, Removed: true}); err != nil {
					return err
				}
				// The previously announced pod is not present in the current list and is already announced as `Removed`.
				delete(announced, k)
			}
		}

		for _, key := range keys {
			instance := key.Instance
			if _, ok := newM[instance.UniqueID()]; !ok {
				continue
			}
			human := labels[instance.UniqueID()]
			if human == "" {
				human = instance.HumanReference()
			}

			if err := onInstance(runtime.ObserveEvent{ContainerReference: instance, HumanReadableID: human, Added: true}); err != nil {
				return err
			}
			announced[instance.UniqueID()] = instance
		}

		if opts.OneShot {
			return nil
		}

		time.Sleep(1 * time.Second)
	}
}
