// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubeobserver

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/tasks"
)

type ConditionWaiter[Client any] interface {
	Prepare(context.Context, Client) error
	Poll(context.Context, Client) (bool, error)
}

func WaitForCondition[Client any](ctx context.Context, cli Client, action *tasks.ActionEvent, waiter ConditionWaiter[Client]) error {
	return action.Run(ctx, func(ctx context.Context) error {
		if err := waiter.Prepare(ctx, cli); err != nil {
			return err
		}

		return client.PollImmediateWithContext(ctx, 500*time.Millisecond, 5*time.Minute, func(ctx context.Context) (bool, error) {
			return waiter.Poll(ctx, cli)
		})
	})
}

type WaitOnResource struct {
	RestConfig *rest.Config

	Name, Namespace string
	Description     string
	ResourceKind    string
	Scope           schema.PackageName

	PreviousGen, ExpectedGen int64
}

func (w WaitOnResource) WaitUntilReady(ctx context.Context, ch chan ops.Event) error {
	if ch != nil {
		defer close(ch)
	}

	cli, err := k8s.NewForConfig(w.RestConfig)
	if err != nil {
		return err
	}

	ev := tasks.Action(runtime.TaskServerStart)
	if w.Scope != "" {
		ev = ev.Scope(w.Scope)
	} else {
		ev = ev.Arg("kind", w.ResourceKind).Arg("name", w.Name).Arg("namespace", w.Namespace)
	}

	return ev.Run(ctx, func(ctx context.Context) error {
		ev := ops.Event{
			ResourceID:          fmt.Sprintf("%s/%s", w.Namespace, w.Name),
			Kind:                w.ResourceKind,
			Scope:               w.Scope,
			RuntimeSpecificHelp: fmt.Sprintf("kubectl -n %s describe %s %s", w.Namespace, strings.ToLower(w.ResourceKind), w.Name),
		}

		switch w.ResourceKind {
		case "Deployment", "StatefulSet":
			ev.Category = "Servers deployed"
		default:
			ev.Category = w.Description
		}

		if w.PreviousGen == w.ExpectedGen {
			ev.AlreadyExisted = true
		}

		if ch != nil {
			ch <- ev
		}

		return client.PollImmediateWithContext(ctx, 500*time.Millisecond, 5*time.Minute, func(c context.Context) (done bool, err error) {
			var observedGeneration int64
			var readyReplicas, replicas int32

			switch w.ResourceKind {
			case "Deployment":
				res, err := cli.AppsV1().Deployments(w.Namespace).Get(c, w.Name, metav1.GetOptions{})
				if err != nil {
					return false, err
				}

				observedGeneration = res.Status.ObservedGeneration
				replicas = res.Status.Replicas
				readyReplicas = res.Status.ReadyReplicas
				ev.ImplMetadata = res.Status

			case "StatefulSet":
				res, err := cli.AppsV1().StatefulSets(w.Namespace).Get(c, w.Name, metav1.GetOptions{})
				if err != nil {
					return false, err
				}

				observedGeneration = res.Status.ObservedGeneration
				replicas = res.Status.Replicas
				readyReplicas = res.Status.ReadyReplicas
				ev.ImplMetadata = res.Status

			default:
				return false, fnerrors.InternalError("%s: unsupported resource type for watching", w.ResourceKind)
			}

			if rs, err := fetchReplicaSetName(c, cli, w.Namespace, w.Name, w.ExpectedGen); err == nil {
				if status, err := podWaitingStatus(c, cli, w.Namespace, rs); err == nil {
					ev.WaitStatus = status
				}
			}

			ev.Ready = ops.NotReady
			if observedGeneration > w.ExpectedGen {
				ev.Ready = ops.Ready
			} else if observedGeneration == w.ExpectedGen {
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

type podWaiter struct {
	selector func(context.Context, *k8s.Clientset) ([]corev1.Pod, error)
	isOk     func(corev1.PodStatus) (bool, error)

	mu                   sync.Mutex
	podCount, matchCount int
}

// FormatProgress implements ActionProgress.
func (w *podWaiter) FormatProgress() string {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.podCount == 0 {
		return "(waiting for pods...)"
	}

	return fmt.Sprintf("%d / %d", w.matchCount, w.podCount)
}

func (w *podWaiter) Prepare(ctx context.Context, c *k8s.Clientset) error {
	tasks.Attachments(ctx).SetProgress(w)
	return nil
}

func (w *podWaiter) Poll(ctx context.Context, c *k8s.Clientset) (bool, error) {
	pods, err := w.selector(ctx, c)
	if err != nil {
		return false, err
	}

	var count int
	for _, pod := range pods {
		// If the pod is configured to never restart, we check if it's in an unrecoverable state.
		if pod.Spec.RestartPolicy == corev1.RestartPolicyNever {
			var terminated [][2]string
			for _, init := range pod.Status.InitContainerStatuses {
				if init.State.Terminated != nil && init.State.Terminated.ExitCode != 0 {
					terminated = append(terminated, [2]string{
						init.Name,
						fmt.Sprintf("%s: exit code %d", init.State.Terminated.Reason, init.State.Terminated.ExitCode),
					})
				}
			}

			for _, container := range pod.Status.ContainerStatuses {
				if container.State.Terminated != nil && container.State.Terminated.ExitCode != 0 {
					terminated = append(terminated, [2]string{
						container.Name,
						fmt.Sprintf("%s: exit code %d", container.State.Terminated.Reason, container.State.Terminated.ExitCode),
					})
				}
			}

			if len(terminated) > 0 {
				var failed []runtime.ContainerReference
				var labels []string
				for _, t := range terminated {
					labels = append(labels, fmt.Sprintf("%s: %s", t[0], t[1]))
					failed = append(failed, kubedef.ContainerPodReference{
						Namespace: pod.Namespace,
						PodName:   pod.Name,
						Container: t[0],
					})
				}

				return false, runtime.ErrContainerFailed{
					Name:             fmt.Sprintf("%s/%s", pod.Namespace, pod.Name),
					Reason:           strings.Join(labels, "; "),
					FailedContainers: failed,
				}
			}
		}

		ok, err := w.isOk(pod.Status)
		if err != nil {
			return false, err
		}
		if ok {
			count++
			break // Don't overcount.
		}
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	w.podCount = len(pods)
	w.matchCount = count

	return count > 0 && count == len(pods), nil
}

func WaitForPodConditition(selector func(context.Context, *k8s.Clientset) ([]corev1.Pod, error), isOk func(corev1.PodStatus) (bool, error)) ConditionWaiter[*k8s.Clientset] {
	return &podWaiter{selector: selector, isOk: isOk}
}

func MatchPodCondition(typ corev1.PodConditionType) func(corev1.PodStatus) (bool, error) {
	return func(ps corev1.PodStatus) (bool, error) {
		return matchPodCondition(ps, typ), nil
	}
}

func matchPodCondition(ps corev1.PodStatus, typ corev1.PodConditionType) bool {
	for _, cond := range ps.Conditions {
		if cond.Type == typ && cond.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func SelectPods(ns string, name *string, selector map[string]string) func(context.Context, *k8s.Clientset) ([]corev1.Pod, error) {
	sel := kubedef.SerializeSelector(selector)

	return func(ctx context.Context, c *k8s.Clientset) ([]corev1.Pod, error) {
		pods, err := c.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{LabelSelector: sel})
		if err != nil {
			return nil, err
		}

		if name != nil {
			var filtered []corev1.Pod
			for _, item := range pods.Items {
				if item.GetName() == *name {
					filtered = append(filtered, item)
				}
			}
			return filtered, nil
		}

		return pods.Items, nil
	}
}
