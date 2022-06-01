// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubernetes

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	k8s "k8s.io/client-go/kubernetes"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/tasks"
	"namespacelabs.dev/go-ids"
)

func (r K8sRuntime) RunOneShot(ctx context.Context, pkg schema.PackageName, runOpts runtime.ServerRunOpts, logOutput io.Writer) error {
	parts := strings.Split(pkg.String(), "/")

	name := strings.ToLower(parts[len(parts)-1]) + "-" + ids.NewRandomBase32ID(8)

	cli, err := client.NewClientFromHostEnv(ctx, r.hostEnv)
	if err != nil {
		return err
	}

	spec, err := makePodSpec(name, runOpts)
	if err != nil {
		return err
	}

	if err := spawnAndWaitPod(ctx, cli, r.moduleNamespace, name, spec, false); err != nil {
		return err
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
			var e runtime.ErrContainerFailed
			if errors.As(err, &e) {
				return err
			}

			return fnerrors.InternalError("kubernetes: expected pod to have terminated, but didn't see termination status: %w", err)
		}
	}
}

func (r K8sRuntime) RunAttached(ctx context.Context, name string, runOpts runtime.ServerRunOpts, io runtime.TerminalIO) error {
	return r.RunAttachedOpts(ctx, name, runOpts, io, nil)
}

func (r K8sRuntime) RunAttachedOpts(ctx context.Context, name string, runOpts runtime.ServerRunOpts, io runtime.TerminalIO, onStart func()) error {
	cli, err := client.NewClientFromHostEnv(ctx, r.hostEnv)
	if err != nil {
		return err
	}

	spec, err := makePodSpec(name, runOpts)
	if err != nil {
		return err
	}

	if io.Stdin != nil {
		spec.Containers[0].WithStdin(true).WithStdinOnce(true)
	}

	if io.TTY {
		spec.Containers[0].WithTTY(true)
	}

	if err := spawnAndWaitPod(ctx, cli, r.moduleNamespace, name, spec, true); err != nil {
		if logsErr := fetchPodLogs(ctx, cli, console.TypedOutput(ctx, name, console.CatOutputTool), r.moduleNamespace, name, "", runtime.StreamLogsOpts{}); logsErr != nil {
			fmt.Fprintf(console.Errors(ctx), "Failed to fetch failed container logs: %v\n", logsErr)
		}
		return err
	}

	compute.On(ctx).Cleanup(tasks.Action("kubernetes.pod.delete"), func(ctx context.Context) error {
		return cli.CoreV1().Pods(r.moduleNamespace).Delete(context.Background(), name, metav1.DeleteOptions{})
	})

	if onStart != nil {
		onStart()
	}

	return r.attachTerminal(ctx, cli, containerPodReference{Namespace: r.moduleNamespace, PodName: name}, io)
}

func makePodSpec(name string, runOpts runtime.ServerRunOpts) (*applycorev1.PodSpecApplyConfiguration, error) {
	container := applycorev1.Container().
		WithName(name).
		WithImage(runOpts.Image.RepoAndDigest()).
		WithArgs(runOpts.Args...).
		WithCommand(runOpts.Command...).
		WithSecurityContext(
			applycorev1.SecurityContext().
				WithReadOnlyRootFilesystem(runOpts.ReadOnlyFilesystem))

	if _, err := fillEnv(container, runOpts.Env); err != nil {
		return nil, err
	}

	podSpec := applycorev1.PodSpec().WithContainers(container)
	podSpecSecCtx, err := runAsToPodSecCtx(&applycorev1.PodSecurityContextApplyConfiguration{}, runOpts.RunAs)
	if err != nil {
		return nil, err
	}

	return podSpec.WithSecurityContext(podSpecSecCtx), nil
}

func spawnAndWaitPod(ctx context.Context, cli *k8s.Clientset, ns, name string, container *applycorev1.PodSpecApplyConfiguration, allErrors bool) error {
	pod := applycorev1.Pod(name, ns).
		WithSpec(container.WithRestartPolicy(corev1.RestartPolicyNever)).
		WithLabels(kubedef.SelectNamespaceDriver())

	if _, err := cli.CoreV1().Pods(ns).Apply(ctx, pod, kubedef.Ego()); err != nil {
		return err
	}

	if err := waitForCondition(ctx, cli, tasks.Action("kubernetes.pod.deploy").Arg("name", name),
		WaitForPodConditition(fetchPod(ns, name), func(status corev1.PodStatus) (bool, error) {
			return (status.Phase == corev1.PodRunning || status.Phase == corev1.PodFailed || status.Phase == corev1.PodSucceeded), nil
		})); err != nil {
		if allErrors {
			return err
		}

		if _, ok := err.(runtime.ErrContainerFailed); !ok {
			return err
		}
	}

	if ctx.Err() != nil {
		return ctx.Err()
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
