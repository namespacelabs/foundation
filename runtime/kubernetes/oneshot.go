// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubernetes

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	k8s "k8s.io/client-go/kubernetes"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubeobserver"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func (r *ClusterNamespace) RunOneShot(ctx context.Context, name string, runOpts runtime.ContainerRunOpts, logOutput io.Writer, follow bool) error {
	return r.cluster.RunOneShot(ctx, r.target.namespace, name, runOpts, logOutput, follow)
}

func (r *Cluster) RunOneShot(ctx context.Context, namespace, name string, runOpts runtime.ContainerRunOpts, logOutput io.Writer, follow bool) error {
	spec, err := makePodSpec(name, runOpts)
	if err != nil {
		return err
	}

	if err := spawnAndWaitPod(ctx, r.cli, namespace, name, spec, false); err != nil {
		return err
	}

	defer func() {
		if err := r.cli.CoreV1().Pods(namespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
			fmt.Fprintf(console.Warnings(ctx), "Failed to delete pod %s/%s: %v\n", namespace, name, err)
		}
	}()

	if follow {
		if err := fetchPodLogs(ctx, r.cli, logOutput, namespace, name, "", runtime.FetchLogsOpts{Follow: true}); err != nil {
			return err
		}
	}

	for k := 0; ; k++ {
		finalState, err := r.cli.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return fnerrors.InvocationError("kubernetes: failed to fetch final pod status: %w", err)
		}

		for _, containerStatus := range finalState.Status.ContainerStatuses {
			if term := containerStatus.State.Terminated; term != nil {
				var logErr error
				if k > 0 || !follow {
					if follow {
						fmt.Fprintln(logOutput, "<Attempting to fetch the last 50 lines of test log.>")
					}

					ctxWithTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
					defer cancel()

					opts := runtime.FetchLogsOpts{}
					if follow {
						opts.TailLines = 50
					}

					logErr = fetchPodLogs(ctxWithTimeout, r.cli, logOutput, namespace, name, "", opts)
				}

				if term.ExitCode != 0 {
					return runtime.ErrContainerExitStatus{ExitCode: term.ExitCode}
				}

				return logErr
			}
		}

		if follow {
			fmt.Fprintln(logOutput, "<No longer streaming pod logs, but pod is still running, waiting for completion.>")
		}

		if err := kubeobserver.WaitForCondition(ctx, r.cli,
			tasks.Action("kubernetes.pod.wait").Arg("namespace", namespace).Arg("name", name).Arg("condition", "terminated"),
			kubeobserver.WaitForPodConditition(
				kubeobserver.PickPod(namespace, name),
				func(status corev1.PodStatus) (bool, error) {
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

func (r *ClusterNamespace) RunAttached(ctx context.Context, name string, runOpts runtime.ContainerRunOpts, io runtime.TerminalIO) error {
	return r.cluster.RunAttachedOpts(ctx, r.target.namespace, name, runOpts, io, nil)
}

func (r *Cluster) RunAttachedOpts(ctx context.Context, ns, name string, runOpts runtime.ContainerRunOpts, io runtime.TerminalIO, onStart func()) error {
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

	if err := spawnAndWaitPod(ctx, r.cli, ns, name, spec, true); err != nil {
		if logsErr := fetchPodLogs(ctx, r.cli, console.TypedOutput(ctx, name, console.CatOutputTool), ns, name, "", runtime.FetchLogsOpts{}); logsErr != nil {
			fmt.Fprintf(console.Errors(ctx), "Failed to fetch failed container logs: %v\n", logsErr)
		}
		return err
	}

	compute.On(ctx).Cleanup(tasks.Action("kubernetes.pod.delete").Arg("namespace", ns).Arg("name", name), func(ctx context.Context) error {
		return r.cli.CoreV1().Pods(ns).Delete(context.Background(), name, metav1.DeleteOptions{})
	})

	if onStart != nil {
		onStart()
	}

	return r.attachTerminal(ctx, r.cli, &kubedef.ContainerPodReference{Namespace: ns, PodName: name}, io)
}

func makePodSpec(name string, runOpts runtime.ContainerRunOpts) (*applycorev1.PodSpecApplyConfiguration, error) {
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

	if err := kubeobserver.WaitForCondition(ctx, cli, tasks.Action("kubernetes.pod.deploy").Arg("namespace", ns).Arg("name", name),
		kubeobserver.WaitForPodConditition(kubeobserver.PickPod(ns, name), func(status corev1.PodStatus) (bool, error) {
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
