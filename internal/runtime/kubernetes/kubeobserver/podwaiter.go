// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package kubeobserver

import (
	"context"
	"fmt"
	"sync"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s "k8s.io/client-go/kubernetes"
	"namespacelabs.dev/foundation/framework/kubernetes/kubedef"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/std/tasks"
)

type podWaiter struct {
	namespace string
	selector  metav1.ListOptions
	isOk      func(corev1.PodStatus) (bool, error)

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
	list, err := c.CoreV1().Pods(w.namespace).List(ctx, w.selector)
	if err != nil {
		return false, err
	}

	var count int
	for _, pod := range list.Items {
		// If the pod is configured to never restart, we check if it's in an unrecoverable state.
		if pod.Spec.RestartPolicy == corev1.RestartPolicyNever {
			var failures []runtime.ErrContainerFailed_Failure
			for _, init := range pod.Status.InitContainerStatuses {
				if init.State.Terminated != nil && init.State.Terminated.ExitCode != 0 {
					failures = append(failures, runtime.ErrContainerFailed_Failure{
						Reference: kubedef.MakePodRef(pod.Namespace, pod.Name, init.Name, nil),
						Reason:    init.State.Terminated.Reason,
						Message:   init.State.Terminated.Message,
						ExitCode:  init.State.Terminated.ExitCode,
					})
				}
			}

			for _, container := range pod.Status.ContainerStatuses {
				if container.State.Terminated != nil && container.State.Terminated.ExitCode != 0 {
					failures = append(failures, runtime.ErrContainerFailed_Failure{
						Reference: kubedef.MakePodRef(pod.Namespace, pod.Name, container.Name, nil),
						Reason:    container.State.Terminated.Reason,
						Message:   container.State.Terminated.Message,
						ExitCode:  container.State.Terminated.ExitCode,
					})
				}
			}

			if len(failures) > 0 {
				return false, runtime.ErrContainerFailed{
					Name:     fmt.Sprintf("%s/%s", pod.Namespace, pod.Name),
					Failures: failures,
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

	w.podCount = len(list.Items)
	w.matchCount = count

	return w.matchCount > 0 && w.matchCount == w.podCount, nil
}

func WaitForPodConditition(namespace string, selector metav1.ListOptions, isOk func(corev1.PodStatus) (bool, error)) ConditionWaiter[*k8s.Clientset] {
	return NewPodCondititionWaiter(namespace, selector, isOk)
}

func NewPodCondititionWaiter(namespace string, selector metav1.ListOptions, isOk func(corev1.PodStatus) (bool, error)) *podWaiter {
	return &podWaiter{namespace: namespace, selector: selector, isOk: isOk}
}

func MatchPodCondition(ps corev1.PodStatus, typ corev1.PodConditionType) (corev1.PodCondition, bool) {
	for _, cond := range ps.Conditions {
		if cond.Type == typ && cond.Status == corev1.ConditionTrue {
			return cond, true
		}
	}

	return corev1.PodCondition{}, false
}
