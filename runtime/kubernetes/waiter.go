// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubernetes

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	k8s "k8s.io/client-go/kubernetes"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/tasks"
)

type waitOn struct {
	devHost *schema.DevHost
	env     *schema.Environment

	def      *schema.Definition
	apply    *kubedef.OpApply
	resource string
	scope    schema.PackageName

	previousGen, expectedGen int64
}

func (w waitOn) WaitUntilReady(ctx context.Context, ch chan ops.Event) error {
	if ch != nil {
		defer func() {
			ch <- ops.Event{AllDone: true}
			close(ch)
		}()
	}

	return tasks.Task(runtime.TaskServerStart).Scope(w.scope).Run(ctx,
		func(ctx context.Context) error {
			ev := ops.Event{
				ResourceID: w.apply.Name,
				Kind:       w.apply.Resource,
				Scope:      w.scope,
			}

			switch w.resource {
			case "deployments", "statefulsets":
				ev.Category = "Servers deployed"
			default:
				ev.Category = w.def.Description
			}

			if w.previousGen == w.expectedGen {
				ev.AlreadyExisted = true
			}

			if ch != nil {
				ch <- ev
			}

			cfg, err := client.ComputeHostEnv(w.devHost, w.env)
			if err != nil {
				return err
			}

			cli, err := client.NewClientFromHostEnv(cfg)
			if err != nil {
				return err
			}

			return wait.PollImmediateWithContext(ctx, 500*time.Millisecond, 5*time.Minute, func(c context.Context) (done bool, err error) {
				var observedGeneration int64
				var readyReplicas, replicas int32

				switch w.resource {
				case "deployments":
					res, err := cli.AppsV1().Deployments(w.apply.Namespace).Get(c, w.apply.Name, metav1.GetOptions{})
					if err != nil {
						return false, err
					}

					observedGeneration = res.Status.ObservedGeneration
					replicas = res.Status.Replicas
					readyReplicas = res.Status.ReadyReplicas
					ev.ImplMetadata = res.Status

				case "statefulsets":
					res, err := cli.AppsV1().StatefulSets(w.apply.Namespace).Get(c, w.apply.Name, metav1.GetOptions{})
					if err != nil {
						return false, err
					}

					observedGeneration = res.Status.ObservedGeneration
					replicas = res.Status.Replicas
					readyReplicas = res.Status.ReadyReplicas
					ev.ImplMetadata = res.Status

				default:
					return false, fnerrors.InternalError("%s: unsupported resource type for watching", w.resource)
				}

				ev.Ready = ops.NotReady
				if observedGeneration > w.expectedGen {
					ev.Ready = ops.Ready
				} else if observedGeneration == w.expectedGen {
					if readyReplicas == replicas && replicas > 0 {
						ev.Ready = ops.Ready
					}
				}

				if ch != nil {
					ch <- ev
				}

				return ev.Ready == ops.Ready, nil
			})
		})
}

func WaitForPodConditition(selector func(context.Context, *k8s.Clientset) ([]corev1.Pod, error), isOk func(corev1.PodStatus) bool) func(context.Context, *k8s.Clientset) (bool, error) {
	return func(ctx context.Context, c *k8s.Clientset) (bool, error) {
		pods, err := selector(ctx, c)
		if err != nil {
			return false, err
		}

		var count int
		for _, pod := range pods {
			if isOk(pod.Status) {
				count++
				break // Don't overcount.
			}
		}

		tasks.Attachments(ctx).AddResult("pod_count", len(pods)).AddResult("match_count", count)

		return count > 0 && count == len(pods), nil
	}
}

func MatchPodCondition(typ corev1.PodConditionType) func(corev1.PodStatus) bool {
	return func(ps corev1.PodStatus) bool {
		for _, cond := range ps.Conditions {
			if cond.Type == typ && cond.Status == corev1.ConditionTrue {
				return true
			}
		}
		return false
	}
}

func SelectPods(ns string, selector map[string]string) func(context.Context, *k8s.Clientset) ([]corev1.Pod, error) {
	sel := kubedef.SerializeSelector(selector)

	return func(ctx context.Context, c *k8s.Clientset) ([]corev1.Pod, error) {
		pods, err := c.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{LabelSelector: sel})
		if err != nil {
			return nil, err
		}

		return pods.Items, nil
	}
}