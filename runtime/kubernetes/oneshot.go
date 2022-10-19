// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubernetes

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	k8s "k8s.io/client-go/kubernetes"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubeobserver"
	"namespacelabs.dev/foundation/std/tasks"
)

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

	if io.Stdin != nil || io.Stdout != nil || io.Stderr != nil {
		return r.attachTerminal(ctx, r.cli, &kubedef.ContainerPodReference{Namespace: ns, PodName: name}, io)
	}
	return nil
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

	if _, err := fillEnv(container, runOpts.Env, runtime.GroundedSecrets{}, nil); err != nil {
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
		kubeobserver.WaitForPodConditition(ns, kubeobserver.PickPod(name), func(status corev1.PodStatus) (bool, error) {
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
