// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubernetes

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	"golang.org/x/exp/slices"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	k8s "k8s.io/client-go/kubernetes"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/tasks"
	"namespacelabs.dev/go-ids"
)

// podResolver continuously attempts to resolve a single pod that match the specified set of labels.
// If the resolved pod is terminated, a new one is picked.
type podResolver struct {
	client    *k8s.Clientset
	namespace string
	labels    map[string]string

	mu          sync.Mutex
	cond        *sync.Cond
	revision    int64
	runningPods []v1.Pod
	watchers    []watcherRegistration
}

type watcherRegistration struct {
	id string
	f  func(*v1.Pod, int64, error)
}

func newPodResolver(client *k8s.Clientset, namespace string, labels map[string]string) *podResolver {
	p := &podResolver{
		client:    client,
		namespace: namespace,
		labels:    labels,
	}
	p.cond = sync.NewCond(&p.mu)
	return p
}

func (p *podResolver) Start(ctx context.Context) {
	compute.On(ctx).Detach(tasks.Action("kubernetes.pod-resolver").Indefinite(), func(rootCtx context.Context) error {
		// Note: the passed in ctx is used instead, as we want to react to cancelations.

		sel := kubedef.SerializeSelector(p.labels)
		w, err := p.client.CoreV1().Pods(p.namespace).Watch(ctx, metav1.ListOptions{LabelSelector: sel})
		if err != nil {
			return err
		}

		defer w.Stop()
		defer p.cond.Broadcast() // On exit, wake up all waiters.

		debug := console.Debug(ctx)

		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case ev, ok := <-w.ResultChan():
				if !ok {
					fmt.Fprintf(debug, "kube/podresolver: %s: closed\n", sel)
					return fnerrors.New("unexpected watch closure")
				}

				if ev.Object == nil {
					continue
				}

				pod, ok := ev.Object.(*v1.Pod)
				if !ok {
					fmt.Fprintf(debug, "kube/podresolver: %s: received non-pod event: %v\n", sel, reflect.TypeOf(ev.Object))
					continue
				}

				fmt.Fprintf(debug, "kube/podresolver: %s: event type %s: name %s phase %s\n",
					sel, ev.Type, pod.Name, pod.Status.Phase)

				p.mu.Lock()
				existingIndex := slices.IndexFunc(p.runningPods, func(existing v1.Pod) bool {
					return existing.UID == pod.UID
				})

				var modified bool
				if ev.Type == watch.Deleted || pod.Status.Phase != v1.PodRunning {
					if existingIndex >= 0 {
						p.runningPods = slices.Delete(p.runningPods, existingIndex, existingIndex+1)
						modified = true
						fmt.Fprintf(debug, "kube/podresolver: %s: remove pod-uid %s\n", sel, pod.UID)
					}
				} else if (ev.Type == watch.Added || ev.Type == watch.Modified) && pod.Status.Phase == v1.PodRunning {
					if existingIndex < 0 {
						p.runningPods = append(p.runningPods, *pod)
						modified = true
						fmt.Fprintf(debug, "kube/podresolver: %s: add pod-uid %s\n", sel, pod.UID)
					}
				}

				if modified {
					p.revision++
					p.cond.Broadcast()
					p.broadcast()
				}
				p.mu.Unlock()
			}
		}
	})
}

func (p *podResolver) Watch(f func(*v1.Pod, int64, error)) func() {
	id := ids.NewRandomBase32ID(8)

	p.mu.Lock()
	defer p.mu.Unlock()

	p.watchers = append(p.watchers, watcherRegistration{id, f})
	f(p.selectPod(), p.revision, nil)

	return func() {
		p.mu.Lock()
		defer p.mu.Unlock()

		index := slices.IndexFunc(p.watchers, func(e watcherRegistration) bool {
			return e.id == id
		})

		if index >= 0 {
			p.watchers = slices.Delete(p.watchers, index, index+1)
		}
	}
}

func (p *podResolver) selectPod() *v1.Pod {
	if len(p.runningPods) > 0 {
		return &p.runningPods[0]
	}
	return nil
}

func (p *podResolver) broadcast() {
	pod := p.selectPod()
	for _, w := range p.watchers {
		w.f(pod, p.revision, nil)
	}
}

func (p *podResolver) Wait(ctx context.Context) (v1.Pod, error) {
	return cancelableWait(ctx, p.cond, func() (v1.Pod, bool) {
		pod := p.selectPod()
		if pod == nil {
			return v1.Pod{}, false
		}
		return *pod, true
	})
}

func cancelableWait[V any](ctx context.Context, cond *sync.Cond, resolve func() (V, bool)) (V, error) {
	cond.L.Lock()
	defer cond.L.Unlock()

	for {
		v, ok := resolve()
		if !ok {
			// Has the context been canceled?
			if err := ctx.Err(); err != nil {
				return v, nil
			}
		} else {
			return v, nil
		}

		cond.Wait()
	}
}
