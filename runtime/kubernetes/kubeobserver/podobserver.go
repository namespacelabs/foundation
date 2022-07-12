// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubeobserver

import (
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"
	"sync"
	"time"

	"golang.org/x/exp/slices"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	k8s "k8s.io/client-go/kubernetes"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/go-ids"
)

// PodObserver continuously attempts to resolve a single pod that match the specified set of labels.
// If the resolved pod is terminated, a new one is picked.
type PodObserver struct {
	client    *k8s.Clientset
	namespace string
	labels    map[string]string

	mu           sync.Mutex
	cond         *sync.Cond
	revision     int64
	runningPods  []v1.Pod
	watchers     []watcherRegistration
	permanentErr error
}

type watcherRegistration struct {
	id    string
	onPod func(*v1.Pod, int64, error)
}

func NewPodObserver(ctx context.Context, client *k8s.Clientset, namespace string, labels map[string]string) *PodObserver {
	p := &PodObserver{
		client:    client,
		namespace: namespace,
		labels:    labels,
	}
	p.cond = sync.NewCond(&p.mu)
	p.start(ctx)
	return p
}

func (p *PodObserver) start(ctx context.Context) {
	go func() {
		defer p.cond.Broadcast() // On exit, wake up all waiters.

		debug := console.Debug(ctx)
		for {
			retry, err := p.runWatcher(ctx, debug)
			if err == nil {
				return
			}

			if retry {
				fmt.Fprintf(console.Debug(ctx), "kube/podresolver: retrying: %v.\n", err)
				// XXX exponential back-off?
				time.Sleep(2 * time.Second)
			} else {
				p.mu.Lock()
				p.permanentErr = err
				p.mu.Unlock()

				if !errors.Is(err, context.Canceled) {
					fmt.Fprintf(console.Errors(ctx), "kube/podresolver: failed: %v.\n", err)
				}

				return
			}
		}
	}()
}

// Return true for a retry.
func (p *PodObserver) runWatcher(ctx context.Context, debug io.Writer) (bool, error) {
	sel := kubedef.SerializeSelector(p.labels)

	w, err := p.client.CoreV1().Pods(p.namespace).Watch(ctx, metav1.ListOptions{LabelSelector: sel})
	if err != nil {
		return true, err
	}

	defer w.Stop()

	for {
		select {
		case <-ctx.Done():
			return false, ctx.Err()

		case ev, ok := <-w.ResultChan():
			if !ok {
				fmt.Fprintf(debug, "kube/podresolver: %s: closed.\n", sel)
				return true, fnerrors.New("unexpected watch closure, will retry")
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
}

func (p *PodObserver) Watch(f func(*v1.Pod, int64, error)) func() {
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

func (p *PodObserver) selectPod() *v1.Pod {
	if len(p.runningPods) > 0 {
		// Always pick the last pod, as that's the most recent to show up and is
		// likely to be the one that survives e.g. a new deployment.
		return &p.runningPods[len(p.runningPods)-1]
	}
	return nil
}

func (p *PodObserver) broadcast() {
	if p.permanentErr != nil {
		for _, w := range p.watchers {
			w.onPod(nil, p.revision, p.permanentErr)
		}
		return
	}

	pod := p.selectPod()
	for _, w := range p.watchers {
		w.onPod(pod, p.revision, nil)
	}
}

func (p *PodObserver) Resolve(ctx context.Context) (v1.Pod, error) {
	return cancelableWait(ctx, p.cond, func() (v1.Pod, bool, error) {
		if p.permanentErr != nil {
			return v1.Pod{}, false, p.permanentErr
		}

		pod := p.selectPod()
		if pod == nil {
			return v1.Pod{}, false, nil
		}
		return *pod, true, nil
	})
}

func cancelableWait[V any](ctx context.Context, cond *sync.Cond, resolve func() (V, bool, error)) (V, error) {
	cond.L.Lock()
	defer cond.L.Unlock()

	for {
		v, ok, err := resolve()
		if err != nil {
			return v, err
		} else if !ok {
			// Has the context been canceled?
			if err := ctx.Err(); err != nil {
				return v, err
			}
		} else {
			return v, nil
		}

		cond.Wait()
	}
}
