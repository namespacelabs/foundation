// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubernetes

import (
	"context"
	"io"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	k8s "k8s.io/client-go/kubernetes"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/runtime/kubernetes/client"
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

	cli, err := client.NewClientFromHostEnv(r.hostEnv)
	if err != nil {
		return err
	}

	pod := applycorev1.Pod(name, r.ns()).
		WithSpec(applycorev1.PodSpec().WithContainers(container).WithRestartPolicy(corev1.RestartPolicyNever))

	if _, err := cli.CoreV1().Pods(r.ns()).Apply(ctx, pod, ego()); err != nil {
		return err
	}

	if err := r.Wait(ctx, tasks.Action("kubernetes.pod.deploy"), WaitForPodConditition(fetchPod(r.ns(), name), func(status corev1.PodStatus) bool {
		return status.Phase == corev1.PodRunning || status.Phase == corev1.PodFailed || status.Phase == corev1.PodSucceeded
	})); err != nil {
		return err
	}

	if err := r.fetchPodLogs(ctx, cli, logOutput, name, "", runtime.StreamLogsOpts{}); err != nil {
		return err
	}

	finalState, err := cli.CoreV1().Pods(r.ns()).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fnerrors.RemoteError("kubernetes: failed to fetch final pod status: %w", err)
	}

	for _, containerStatus := range finalState.Status.ContainerStatuses {
		if term := containerStatus.State.Terminated; term != nil {
			if term.ExitCode != 0 {
				return runtime.ErrContainerExitStatus{ExitCode: term.ExitCode}
			}
		}
	}

	return nil
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