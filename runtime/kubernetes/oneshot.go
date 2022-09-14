// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubernetes

import (
	"context"
	"fmt"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	k8s "k8s.io/client-go/kubernetes"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubeobserver"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func (r *ClusterNamespace) WaitForTermination(ctx context.Context, object runtime.DeployableObject) ([]runtime.ContainerStatus, error) {
	if object.GetDeployableClass() != string(schema.DeployableClass_ONESHOT) {
		return nil, fnerrors.InternalError("WaitForTermination: only support one-shot deployments")
	}

	cli := r.cluster.cli
	namespace := r.target.namespace
	podName := kubedef.MakeDeploymentId(object)

	debug := console.Debug(ctx)

	return tasks.Return(ctx, tasks.Action("kubernetes.wait-for-deployable").Arg("id", object.GetId()).Arg("name", object.GetName()),
		func(ctx context.Context) ([]runtime.ContainerStatus, error) {
			for {
				w, err := cli.CoreV1().Pods(namespace).Watch(ctx, metav1.ListOptions{LabelSelector: kubedef.SerializeSelector(kubedef.SelectById(object))})
				if err != nil {
					return nil, fnerrors.InternalError("kubernetes: failed while waiting for pod: %w", err)
				}

				defer w.Stop()

				for ev := range w.ResultChan() {
					if ev.Object == nil {
						continue
					}

					pod, ok := ev.Object.(*corev1.Pod)
					if !ok {
						fmt.Fprintf(debug, "received non-pod event: %v\n", reflect.TypeOf(ev.Object))
						continue
					}

					if pod.Status.Phase != corev1.PodFailed && pod.Status.Phase != corev1.PodSucceeded {
						continue
					}

					var all []corev1.ContainerStatus
					all = append(all, pod.Status.ContainerStatuses...)
					all = append(all, pod.Status.InitContainerStatuses...)

					var status []runtime.ContainerStatus
					for _, container := range all {
						st := runtime.ContainerStatus{
							Reference: kubedef.MakePodRef(namespace, podName, container.Name, object),
						}

						if container.State.Terminated != nil {
							if container.State.Terminated.ExitCode != 0 {
								st.TerminationError = runtime.ErrContainerExitStatus{ExitCode: container.State.Terminated.ExitCode}
							}
						}

						status = append(status, st)
					}

					return status, nil
				}
			}
		})
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
