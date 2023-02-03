// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package kubernetes

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/anypb"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"namespacelabs.dev/foundation/framework/kubernetes/kubedef"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/common"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes/kubeobserver"
	orchclient "namespacelabs.dev/foundation/orchestration/client"
	"namespacelabs.dev/foundation/schema"
	runtimepb "namespacelabs.dev/foundation/schema/runtime"
	"namespacelabs.dev/foundation/schema/storage"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/tasks"
)

type ClusterNamespace struct {
	parent     runtime.Cluster
	underlying *Cluster
	target     BoundNamespace
}

type BoundNamespace struct {
	env       *schema.Environment
	namespace string
	planning  *client.DeploymentPlanning
}

var _ runtime.ClusterNamespace = &ClusterNamespace{}
var _ kubedef.KubeClusterNamespace = &ClusterNamespace{}

func ConnectToNamespace(ctx context.Context, env cfg.Context) (*ClusterNamespace, error) {
	cluster, err := ConnectToCluster(ctx, env.Configuration())
	if err != nil {
		return nil, err
	}

	bound, err := cluster.Bind(ctx, env)
	if err != nil {
		return nil, err
	}

	return bound.(*ClusterNamespace), nil
}

func (r *ClusterNamespace) KubeConfig() kubedef.KubeConfig {
	return kubedef.KubeConfig{
		Context:     r.underlying.Prepared.HostEnv.GetContext(),
		Environment: r.target.env,
		Namespace:   r.target.namespace,
	}
}

func (cn *ClusterNamespace) Cluster() runtime.Cluster {
	return cn.parent
}

func (r *ClusterNamespace) FetchEnvironmentDiagnostics(ctx context.Context) (*storage.EnvironmentDiagnostics, error) {
	systemInfo, err := r.underlying.SystemInfo(ctx)
	if err != nil {
		return nil, err
	}

	events, err := r.underlying.cli.CoreV1().Events(r.target.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fnerrors.InvocationError("kubernetes", "failed to obtain event list: %w", err)
	}

	// Ignore failures, this is best effort.
	eventsBytes, _ := json.Marshal(events)

	kube := &kubedef.KubernetesEnvDiagnostics{
		SystemInfo:          systemInfo,
		SerializedEventList: string(eventsBytes),
	}

	diag := &storage.EnvironmentDiagnostics{Runtime: "kubernetes"}

	serializedKube, err := anypb.New(kube)
	if err != nil {
		return nil, fnerrors.New("kubernetes: failed to serialize KubernetesEnvDiagnostics")
	}

	diag.RuntimeSpecific = append(diag.RuntimeSpecific, serializedKube)

	return diag, nil
}

func (r *ClusterNamespace) startTerminal(ctx context.Context, cli *kubernetes.Clientset, server runtime.Deployable, rio runtime.TerminalIO, cmd []string) error {
	pod, err := r.resolvePod(ctx, cli, rio.Stderr, server)
	if err != nil {
		return err
	}

	return r.underlying.lowLevelAttachTerm(ctx, cli, pod.Namespace, pod.Name, rio, "exec", &corev1.PodExecOptions{
		Command: cmd,
		Stdin:   rio.Stdin != nil,
		Stdout:  rio.Stdout != nil,
		Stderr:  rio.Stderr != nil,
		TTY:     rio.TTY,
	})
}

func shouldLog(start, lastMsg time.Time) bool {
	if lastMsg.IsZero() {
		return time.Since(start) > 5*time.Second
	}

	return time.Since(lastMsg) > 3*time.Second
}

func (r *ClusterNamespace) WaitUntilReady(ctx context.Context, srv runtime.Deployable) error {
	fmt.Fprintf(console.Debug(ctx), "wait-until-ready: asPods: %v deployable: %+v\n", deployAsPods(r.target.env), srv)

	t := time.Now()
	var lastMsg time.Time

	return tasks.Action("deployable.wait-until-ready").
		Scope(srv.GetPackageRef().AsPackageName()).
		Arg("id", srv.GetId()).Run(ctx, func(ctx context.Context) error {
		return client.PollImmediateWithContext(ctx, 500*time.Millisecond, 5*time.Minute, func(ctx context.Context) (bool, error) {
			if ready, err := r.isDeployableReady(ctx, srv); err != nil || !ready {
				return ready, err
			}

			res, err := r.areServicesReady(ctx, srv)
			if err != nil {
				return false, err
			}

			// Hacky output to make service readiness issues more understandable.
			// Ideally, we'd use orchestration events for these, but we'd need to link
			// them to the server that is waiting for the dependency, not the one that we are waited for.
			if res.Message != "" {
				if shouldLog(t, lastMsg) {
					lastMsg = time.Now()
					fmt.Fprintf(console.TypedOutput(ctx, "service readiness", common.CatOutputTool), "%s\n", res.Message)
				} else {
					fmt.Fprintf(console.Debug(ctx), "%s\n", res.Message)
				}
			}

			return res.Ready, nil
		})
	})
}

func (r *ClusterNamespace) isDeployableReady(ctx context.Context, srv runtime.Deployable) (bool, error) {
	if deployAsPods(r.target.env) {
		return r.isPodReady(ctx, srv)
	}

	switch srv.GetDeployableClass() {
	case string(schema.DeployableClass_STATELESS):
		deployment, err := r.underlying.cli.AppsV1().Deployments(r.target.namespace).Get(ctx, kubedef.MakeDeploymentId(srv), metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		replicas := deployment.Status.Replicas
		readyReplicas := deployment.Status.ReadyReplicas
		updatedReplicas := deployment.Status.UpdatedReplicas

		return kubeobserver.AreReplicasReady(replicas, readyReplicas, updatedReplicas), nil

	case string(schema.DeployableClass_STATEFUL):
		deployment, err := r.underlying.cli.AppsV1().StatefulSets(r.target.namespace).Get(ctx, kubedef.MakeDeploymentId(srv), metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		replicas := deployment.Status.Replicas
		readyReplicas := deployment.Status.ReadyReplicas
		updatedReplicas := deployment.Status.UpdatedReplicas

		return kubeobserver.AreReplicasReady(replicas, readyReplicas, updatedReplicas), nil

	case string(schema.DeployableClass_DAEMONSET):
		deployment, err := r.underlying.cli.AppsV1().DaemonSets(r.target.namespace).Get(ctx, kubedef.MakeDeploymentId(srv), metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		return deployment.Status.NumberReady > 0 && deployment.Status.NumberReady == deployment.Status.NumberAvailable, nil

	case string(schema.DeployableClass_MANUAL), string(schema.DeployableClass_ONESHOT):
		return r.isPodReady(ctx, srv)

	default:
		return false, fnerrors.InternalError("don't how to check for readiness of %q", srv.GetDeployableClass())
	}
}

func (r *ClusterNamespace) areServicesReady(ctx context.Context, srv runtime.Deployable) (ServiceReadiness, error) {
	if client.IsInclusterClient(r.underlying.cli) {
		return AreServicesReady(ctx, r.underlying.cli, r.target.namespace, srv)
	}

	if !orchclient.UseOrchestrator {
		fmt.Fprintf(console.Debug(ctx), "will not wait for services of server %s...\n", srv.GetName())
		return ServiceReadiness{Ready: true}, nil
	}

	conn, err := orchclient.ConnectToOrchestrator(ctx, r.parent)
	if err != nil {
		return ServiceReadiness{}, err
	}

	res, err := orchclient.CallAreServicesReady(ctx, conn, srv, r.target.namespace)
	if err != nil {
		if status.Code(err) == codes.Unimplemented {
			fmt.Fprintf(console.Debug(ctx), "old orchestrator version, will not wait for services of server %s...\n", srv.GetName())
			return ServiceReadiness{Ready: true}, nil
		}

		return ServiceReadiness{}, err
	}

	return ServiceReadiness{Ready: res.Ready, Message: res.Message}, nil
}

func (r *ClusterNamespace) isPodReady(ctx context.Context, srv runtime.Deployable) (bool, error) {
	pod, err := r.underlying.cli.CoreV1().Pods(r.target.namespace).Get(ctx, kubedef.MakeDeploymentId(srv), metav1.GetOptions{})
	if err != nil {
		return false, err
	}

	switch pod.Status.Phase {
	case corev1.PodSucceeded:
		return true, nil

	case corev1.PodFailed:
		if err := kubeobserver.CheckContainerFailed(*pod); err != nil {
			return false, err
		}

		return false, fnerrors.New("pod %q failed", pod.Name)
	}

	for _, reason := range pod.Status.Conditions {
		if reason.Type == corev1.PodReady && reason.Status == corev1.ConditionTrue {
			return true, nil
		}
	}

	return false, nil
}

// Return true on the callback to signal you're done observing.
func (r *ClusterNamespace) Observe(ctx context.Context, srv runtime.Deployable, opts runtime.ObserveOpts, onInstance func(runtime.ObserveEvent) (bool, error)) error {
	type Entry struct {
		Instance   *runtimepb.ContainerReference
		Version    string
		Deployable *runtimepb.Deployable
	}

	announced := map[string]Entry{}

	trackContainer := func(pod corev1.Pod, instance *runtimepb.ContainerReference) (bool, error) {
		if _, has := announced[instance.UniqueId]; has {
			return false, nil
		}

		proto := runtime.DeployableToProto(srv)
		entry := Entry{
			Instance:   instance,
			Version:    pod.ResourceVersion,
			Deployable: proto,
		}
		announced[instance.UniqueId] = entry

		return onInstance(runtime.ObserveEvent{
			Deployable:         entry.Deployable,
			ContainerReference: instance,
			Version:            entry.Version,
			Added:              true,
		})
	}

	untrackContainer := func(_ corev1.Pod, instance *runtimepb.ContainerReference) (bool, error) {
		existing, has := announced[instance.UniqueId]
		if !has {
			return false, nil
		}

		delete(announced, instance.UniqueId)

		return onInstance(runtime.ObserveEvent{
			Deployable:         existing.Deployable,
			ContainerReference: instance,
			Version:            existing.Version,
			Removed:            true,
		})
	}

	pods, err := r.underlying.cli.CoreV1().Pods(r.target.namespace).List(ctx, metav1.ListOptions{LabelSelector: kubedef.SerializeSelector(kubedef.SelectById(srv))})
	if err != nil {
		return err
	}

	if len(pods.Items) == 0 {
		return fnerrors.New("%s: no pods to observe", srv.GetName())
	}

	_, err = kubeobserver.WatchPods(ctx, r.underlying.cli, r.target.namespace, kubedef.SelectById(srv), func(pod corev1.Pod) (any, bool, error) {
		instance := kubedef.MakePodRef(r.target.namespace, pod.Name, kubedef.ServerCtrName(srv), srv)

		t := untrackContainer
		if pod.Status.Phase == corev1.PodRunning {
			t = trackContainer
		}

		if done, err := t(pod, instance); err != nil {
			return nil, false, err
		} else if done {
			return nil, true, nil
		}

		if ObserveInitContainerLogs {
			for _, container := range pod.Spec.InitContainers {
				instance := kubedef.MakePodRef(r.target.namespace, pod.Name, container.Name, srv)
				if done, err := t(pod, instance); err != nil {
					return nil, false, err
				} else if done {
					return nil, true, nil
				}
			}
		}

		return nil, false, nil
	})

	return err
}

func (r *ClusterNamespace) WaitForTermination(ctx context.Context, object runtime.Deployable) ([]runtime.ContainerStatus, error) {
	if object.GetDeployableClass() != string(schema.DeployableClass_ONESHOT) && object.GetDeployableClass() != string(schema.DeployableClass_MANUAL) {
		return nil, fnerrors.InternalError("WaitForTermination: only support one-shot deployments")
	}

	cli := r.underlying.cli
	namespace := r.target.namespace
	podName := kubedef.MakeDeploymentId(object)

	return kubeobserver.WatchDeployable(ctx, "deployable.wait-until-done", cli, namespace, object, func(pod corev1.Pod) ([]runtime.ContainerStatus, bool, error) {
		if pod.Status.Phase != corev1.PodFailed && pod.Status.Phase != corev1.PodSucceeded {
			return nil, false, nil
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

		return status, true, nil
	})
}

func (r *ClusterNamespace) ForwardPort(ctx context.Context, server runtime.Deployable, containerPort int32, localAddrs []string, callback runtime.SinglePortForwardedFunc) (io.Closer, error) {
	if containerPort <= 0 {
		return nil, fnerrors.BadInputError("invalid port number: %d", containerPort)
	}

	return r.underlying.RawForwardPort(ctx, server.GetPackageRef().GetPackageName(), r.target.namespace, kubedef.SelectById(server), int(containerPort), localAddrs, callback)
}

func (r *ClusterNamespace) DialServer(ctx context.Context, server runtime.Deployable, port *schema.Endpoint_Port) (net.Conn, error) {
	return r.underlying.RawDialServer(ctx, r.target.namespace, kubedef.SelectById(server), port)
}

func (r *ClusterNamespace) ResolveContainers(ctx context.Context, object runtime.Deployable) ([]*runtimepb.ContainerReference, error) {
	return kubeobserver.WatchDeployable(ctx, "deployable.resolve-containers", r.underlying.cli, r.target.namespace, object,
		func(pod corev1.Pod) ([]*runtimepb.ContainerReference, bool, error) {
			if pod.Status.Phase != corev1.PodRunning && pod.Status.Phase != corev1.PodFailed && pod.Status.Phase != corev1.PodSucceeded {
				return nil, false, nil
			}

			var refs []*runtimepb.ContainerReference

			for _, init := range pod.Status.InitContainerStatuses {
				refs = append(refs, kubedef.MakePodRef(pod.Namespace, pod.Name, init.Name, object))
			}
			for _, container := range pod.Status.ContainerStatuses {
				refs = append(refs, kubedef.MakePodRef(pod.Namespace, pod.Name, container.Name, object))
			}

			return refs, true, nil
		})
}

func (r *ClusterNamespace) resolvePod(ctx context.Context, cli *kubernetes.Clientset, w io.Writer, obj runtime.Deployable) (corev1.Pod, error) {
	return resolvePodByLabels(ctx, cli, w, r.target.namespace, kubedef.SelectById(obj))
}

func (r *ClusterNamespace) DeployedConfigImageID(ctx context.Context, deployable runtime.Deployable) (oci.ImageID, error) {
	return tasks.Return(ctx, tasks.Action("kubernetes.resolve-config-image-id").Scope(deployable.GetPackageRef().AsPackageName()),
		func(ctx context.Context) (oci.ImageID, error) {
			name := kubedef.MakeDeploymentId(deployable)

			var o kubedef.Object
			switch schema.DeployableClass(deployable.GetDeployableClass()) {
			case schema.DeployableClass_STATELESS:
				d, err := r.underlying.cli.AppsV1().Deployments(r.target.namespace).Get(ctx, name, metav1.GetOptions{})
				if err != nil {
					return oci.ImageID{}, fnerrors.InvocationError("kubernetes", "failed to fetch deployment %s: %w", name, err)
				}
				o = d

			case schema.DeployableClass_STATEFUL:
				ss, err := r.underlying.cli.AppsV1().StatefulSets(r.target.namespace).Get(ctx, name, metav1.GetOptions{})
				if err != nil {
					return oci.ImageID{}, fnerrors.InvocationError("kubernetes", "failed to fetch stateful set %s: %w", name, err)
				}
				o = ss

			case schema.DeployableClass_DAEMONSET:
				ds, err := r.underlying.cli.AppsV1().DaemonSets(r.target.namespace).Get(ctx, name, metav1.GetOptions{})
				if err != nil {
					return oci.ImageID{}, fnerrors.InvocationError("kubernetes", "failed to fetch deamon set %s: %w", name, err)
				}
				o = ds

			default:
				return oci.ImageID{}, fnerrors.InternalError("unable to fetch config image id: unsupported deployable class %q", deployable.GetDeployableClass())
			}

			cfgimage, ok := o.GetAnnotations()[kubedef.K8sConfigImage]
			if !ok {
				return oci.ImageID{}, fnerrors.BadInputError("%s: %q is missing as an annotation in %q",
					deployable.GetPackageRef().GetPackageName(), kubedef.K8sConfigImage, name)
			}

			imgid, err := oci.ParseImageID(cfgimage)
			if err != nil {
				return imgid, err
			}

			tasks.Attachments(ctx).AddResult("ref", imgid.ImageRef())

			return imgid, nil
		})
}

func (r *ClusterNamespace) StartTerminal(ctx context.Context, server runtime.Deployable, rio runtime.TerminalIO, command string, rest ...string) error {
	cmd := append([]string{command}, rest...)

	return r.startTerminal(ctx, r.underlying.cli, server, rio, cmd)
}

func (r *ClusterNamespace) DeleteRecursively(ctx context.Context, wait bool) (bool, error) {
	return DeleteAllRecursively(ctx, r.underlying.cli, wait, nil, r.target.namespace)
}

func (r *ClusterNamespace) DeleteDeployable(ctx context.Context, deployable runtime.Deployable) error {
	listOpts := metav1.ListOptions{LabelSelector: kubedef.SerializeSelector(kubedef.SelectById(deployable))}

	switch deployable.GetDeployableClass() {
	case string(schema.DeployableClass_ONESHOT), string(schema.DeployableClass_MANUAL):
		return r.underlying.cli.CoreV1().Pods(r.target.namespace).DeleteCollection(ctx, metav1.DeleteOptions{}, listOpts)

	case string(schema.DeployableClass_STATEFUL):
		return r.underlying.cli.AppsV1().StatefulSets(r.target.namespace).DeleteCollection(ctx, metav1.DeleteOptions{}, listOpts)

	case string(schema.DeployableClass_STATELESS):
		return r.underlying.cli.AppsV1().Deployments(r.target.namespace).DeleteCollection(ctx, metav1.DeleteOptions{}, listOpts)

	case string(schema.DeployableClass_DAEMONSET):
		return r.underlying.cli.AppsV1().DaemonSets(r.target.namespace).DeleteCollection(ctx, metav1.DeleteOptions{}, listOpts)

	default:
		return fnerrors.InternalError("%s: unsupported deployable class", deployable.GetDeployableClass())
	}
}
