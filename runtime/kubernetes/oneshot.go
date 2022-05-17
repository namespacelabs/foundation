// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubernetes

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	k8s "k8s.io/client-go/kubernetes"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/tasks"
	"namespacelabs.dev/go-ids"
)

func (r k8sRuntime) RunOneShot(ctx context.Context, pkg schema.PackageName, runOpts runtime.ServerRunOpts, logOutput io.Writer) error {
	parts := strings.Split(pkg.String(), "/")

	name := strings.ToLower(parts[len(parts)-1]) + "-" + ids.NewRandomBase32ID(8)

	container := applycorev1.Container().
		WithName(name).
		WithImage(runOpts.Image.RepoAndDigest()).
		WithArgs(runOpts.Args...).
		WithCommand(runOpts.Command...).
		WithSecurityContext(
			applycorev1.SecurityContext().
				WithReadOnlyRootFilesystem(runOpts.ReadOnlyFilesystem))

	cli, err := client.NewClientFromHostEnv(ctx, r.hostEnv)
	if err != nil {
		return err
	}

	pod := applycorev1.Pod(name, r.moduleNamespace).
		WithSpec(applycorev1.PodSpec().WithContainers(container).WithRestartPolicy(corev1.RestartPolicyNever)).
		WithLabels(kubedef.SelectNamespaceDriver())

	if _, err := cli.CoreV1().Pods(r.moduleNamespace).Apply(ctx, pod, kubedef.Ego()); err != nil {
		return err
	}

	if err := r.Wait(ctx, tasks.Action("kubernetes.pod.deploy").Scope(pkg),
		WaitForPodConditition(fetchPod(r.moduleNamespace, name), func(status corev1.PodStatus) (bool, error) {
			return (status.Phase == corev1.PodRunning || status.Phase == corev1.PodFailed || status.Phase == corev1.PodSucceeded), nil
		})); err != nil {
		return err
	}

	if ctx.Err() != nil {
		return ctx.Err()
	}

	if err := fetchPodLogs(ctx, cli, logOutput, r.moduleNamespace, name, "", runtime.StreamLogsOpts{Follow: true}); err != nil {
		return err
	}

	for k := 0; ; k++ {
		finalState, err := cli.CoreV1().Pods(r.moduleNamespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return fnerrors.InvocationError("kubernetes: failed to fetch final pod status: %w", err)
		}

		for _, containerStatus := range finalState.Status.ContainerStatuses {
			if term := containerStatus.State.Terminated; term != nil {
				if k > 0 {
					fmt.Fprintln(logOutput, "<Attempting to fetch the last 50 lines of test log.>")

					ctxWithTimeout, cancel := context.WithTimeout(ctx, 3*time.Second)
					defer cancel()

					_ = fetchPodLogs(ctxWithTimeout, cli, logOutput, r.moduleNamespace, name, "", runtime.StreamLogsOpts{TailLines: 50})
				}

				if term.ExitCode != 0 {
					return runtime.ErrContainerExitStatus{ExitCode: term.ExitCode}
				}

				return nil
			}
		}

		fmt.Fprintln(logOutput, "<No longer streaming pod logs, but pod is still running, waiting for completion.>")

		if err := r.Wait(ctx, tasks.Action("kubernetes.pod.wait"), WaitForPodConditition(fetchPod(r.moduleNamespace, name), func(status corev1.PodStatus) (bool, error) {
			return (status.Phase == corev1.PodFailed || status.Phase == corev1.PodSucceeded), nil
		})); err != nil {
			return fnerrors.InternalError("kubernetes: expected pod to have terminated, but didn't see termination status: %w", err)
		}
	}
}

func fetchPod(ns, name string) func(ctx context.Context, c *k8s.Clientset) ([]corev1.Pod, error) {
	return func(ctx context.Context, c *k8s.Clientset) ([]corev1.Pod, error) {
		pod, err := c.CoreV1().Pods(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}

		return []corev1.Pod{*pod}, nil
	}
}
