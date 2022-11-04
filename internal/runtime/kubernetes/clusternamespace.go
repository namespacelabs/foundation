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
	"sort"
	"time"

	"google.golang.org/protobuf/types/known/anypb"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"namespacelabs.dev/foundation/framework/kubernetes/kubedef"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes/kubeobserver"
	"namespacelabs.dev/foundation/schema"
	runtimepb "namespacelabs.dev/foundation/schema/runtime"
	"namespacelabs.dev/foundation/schema/storage"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/tasks"
)

type ClusterNamespace struct {
	cluster *Cluster
	target  clusterTarget
}

type clusterTarget struct {
	env       *schema.Environment
	namespace string
}

var _ runtime.ClusterNamespace = &ClusterNamespace{}
var _ kubedef.KubeClusterNamespace = &ClusterNamespace{}

func ConnectToNamespace(ctx context.Context, env cfg.Context) (*ClusterNamespace, error) {
	cluster, err := ConnectToCluster(ctx, env.Configuration())
	if err != nil {
		return nil, err
	}
	bound, err := cluster.Bind(env)
	if err != nil {
		return nil, err
	}
	return bound.(*ClusterNamespace), nil
}

func (r *ClusterNamespace) KubeConfig() kubedef.KubeConfig {
	return kubedef.KubeConfig{
		Context:     r.cluster.computedClient.HostEnv.GetContext(),
		Environment: r.target.env,
		Namespace:   r.target.namespace,
	}
}

func (cn *ClusterNamespace) Cluster() runtime.Cluster {
	return cn.cluster
}

func (cn *ClusterNamespace) Planner() runtime.Planner {
	return Planner{fetchSystemInfo: cn.cluster.SystemInfo, target: cn.target}
}

func (r *ClusterNamespace) FetchEnvironmentDiagnostics(ctx context.Context) (*storage.EnvironmentDiagnostics, error) {
	systemInfo, err := r.cluster.SystemInfo(ctx)
	if err != nil {
		return nil, err
	}

	events, err := r.cluster.cli.CoreV1().Events(r.target.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fnerrors.New("kubernetes: failed to obtain event list: %w", err)
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

	return r.cluster.lowLevelAttachTerm(ctx, cli, pod.Namespace, pod.Name, rio, "exec", &corev1.PodExecOptions{
		Command: cmd,
		Stdin:   rio.Stdin != nil,
		Stdout:  rio.Stdout != nil,
		Stderr:  rio.Stderr != nil,
		TTY:     rio.TTY,
	})
}

func (r *ClusterNamespace) WaitUntilReady(ctx context.Context, srv runtime.Deployable) error {
	// XXX incorporate service readiness in check as well.

	fmt.Fprintf(console.Debug(ctx), "wait-until-ready: asPods: %v deployable: %+v\n", deployAsPods(r.target.env), srv)

	return tasks.Action("deployable.wait-until-ready").
		Scope(schema.PackageName(srv.GetPackageName())).
		Arg("id", srv.GetId()).Run(ctx, func(ctx context.Context) error {
		return client.PollImmediateWithContext(ctx, 500*time.Millisecond, 5*time.Minute, func(ctx context.Context) (bool, error) {
			if deployAsPods(r.target.env) {
				return r.isPodReady(ctx, srv)
			}

			switch srv.GetDeployableClass() {
			case string(schema.DeployableClass_STATELESS):
				deployment, err := r.cluster.cli.AppsV1().Deployments(r.target.namespace).Get(ctx, kubedef.MakeDeploymentId(srv), metav1.GetOptions{})
				if err != nil {
					return false, err
				}

				replicas := deployment.Status.Replicas
				readyReplicas := deployment.Status.ReadyReplicas
				updatedReplicas := deployment.Status.UpdatedReplicas

				return kubeobserver.AreReplicasReady(replicas, readyReplicas, updatedReplicas), nil

			case string(schema.DeployableClass_STATEFUL):
				deployment, err := r.cluster.cli.AppsV1().StatefulSets(r.target.namespace).Get(ctx, kubedef.MakeDeploymentId(srv), metav1.GetOptions{})
				if err != nil {
					return false, err
				}

				replicas := deployment.Status.Replicas
				readyReplicas := deployment.Status.ReadyReplicas
				updatedReplicas := deployment.Status.UpdatedReplicas

				return kubeobserver.AreReplicasReady(replicas, readyReplicas, updatedReplicas), nil

			case string(schema.DeployableClass_MANUAL), string(schema.DeployableClass_ONESHOT):
				return r.isPodReady(ctx, srv)

			default:
				return false, fnerrors.InternalError("don't how to check for readiness of %q", srv.GetDeployableClass())
			}
		})
	})
}

func (r *ClusterNamespace) isPodReady(ctx context.Context, srv runtime.Deployable) (bool, error) {
	pod, err := r.cluster.cli.CoreV1().Pods(r.target.namespace).Get(ctx, kubedef.MakeDeploymentId(srv), metav1.GetOptions{})
	if err != nil {
		return false, err
	}

	if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
		return true, nil
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
	// XXX use a watch
	announced := map[string]*runtimepb.ContainerReference{}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// No cancelation, moving along.
		}

		pods, err := r.cluster.cli.CoreV1().Pods(r.target.namespace).List(ctx, metav1.ListOptions{
			LabelSelector: kubedef.SerializeSelector(kubedef.SelectById(srv)),
		})
		if err != nil {
			return fnerrors.Wrapf(nil, err, "unable to list pods")
		}

		type Key struct {
			Instance  *runtimepb.ContainerReference
			CreatedAt time.Time // used for sorting
		}

		keys := []Key{}
		newM := map[string]struct{}{}
		labels := map[string]string{}
		for _, pod := range pods.Items {
			if pod.Status.Phase == corev1.PodRunning {
				instance := kubedef.MakePodRef(r.target.namespace, pod.Name, kubedef.ServerCtrName(srv), srv)
				keys = append(keys, Key{
					Instance:  instance,
					CreatedAt: pod.CreationTimestamp.Time,
				})
				newM[instance.UniqueId] = struct{}{}
				labels[instance.UniqueId] = fmt.Sprintf("%s (%s)", srv.GetName(), pod.ResourceVersion)

				if ObserveInitContainerLogs {
					for _, container := range pod.Spec.InitContainers {
						instance := kubedef.MakePodRef(r.target.namespace, pod.Name, container.Name, srv)
						keys = append(keys, Key{Instance: instance, CreatedAt: pod.CreationTimestamp.Time})
						newM[instance.UniqueId] = struct{}{}
						labels[instance.UniqueId] = fmt.Sprintf("%s:%s (%s)", srv.GetName(), container.Name, pod.ResourceVersion)
					}
				}
			}
		}
		sort.SliceStable(keys, func(i int, j int) bool {
			return keys[i].CreatedAt.Before(keys[j].CreatedAt)
		})

		for k, ref := range announced {
			if _, ok := newM[k]; ok {
				delete(newM, k)
			} else {
				if done, err := onInstance(runtime.ObserveEvent{ContainerReference: ref, Removed: true}); err != nil {
					return err
				} else if done {
					return nil
				}
				// The previously announced pod is not present in the current list and is already announced as `Removed`.
				delete(announced, k)
			}
		}

		for _, key := range keys {
			instance := key.Instance
			if _, ok := newM[instance.UniqueId]; !ok {
				continue
			}
			human := labels[instance.UniqueId]
			if human == "" {
				human = instance.HumanReference
			}

			if done, err := onInstance(runtime.ObserveEvent{ContainerReference: instance, HumanReadableID: human, Added: true}); err != nil {
				return err
			} else if done {
				return nil
			}
			announced[instance.UniqueId] = instance
		}

		if opts.OneShot {
			return nil
		}

		time.Sleep(1 * time.Second)
	}
}

func (r *ClusterNamespace) WaitForTermination(ctx context.Context, object runtime.Deployable) ([]runtime.ContainerStatus, error) {
	if object.GetDeployableClass() != string(schema.DeployableClass_ONESHOT) && object.GetDeployableClass() != string(schema.DeployableClass_MANUAL) {
		return nil, fnerrors.InternalError("WaitForTermination: only support one-shot deployments")
	}

	cli := r.cluster.cli
	namespace := r.target.namespace
	podName := kubedef.MakeDeploymentId(object)

	return kubeobserver.WatchDeployable(ctx, "deployable.wait-until-done", cli, namespace, object, func(pod corev1.Pod) ([]runtime.ContainerStatus, bool) {
		if pod.Status.Phase != corev1.PodFailed && pod.Status.Phase != corev1.PodSucceeded {
			return nil, false
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

		return status, true
	})
}

func (r *ClusterNamespace) ForwardPort(ctx context.Context, server runtime.Deployable, containerPort int32, localAddrs []string, callback runtime.SinglePortForwardedFunc) (io.Closer, error) {
	if containerPort <= 0 {
		return nil, fnerrors.BadInputError("invalid port number: %d", containerPort)
	}

	return r.cluster.RawForwardPort(ctx, server.GetPackageName(), r.target.namespace, kubedef.SelectById(server), int(containerPort), localAddrs, callback)
}

func (r *ClusterNamespace) DialServer(ctx context.Context, server runtime.Deployable, containerPort int32) (net.Conn, error) {
	if containerPort <= 0 {
		return nil, fnerrors.BadInputError("invalid port number: %d", containerPort)
	}

	return r.cluster.RawDialServer(ctx, r.target.namespace, kubedef.SelectById(server), int(containerPort))
}

func (r *ClusterNamespace) ResolveContainers(ctx context.Context, object runtime.Deployable) ([]*runtimepb.ContainerReference, error) {
	return kubeobserver.WatchDeployable(ctx, "deployable.resolve-containers", r.cluster.cli, r.target.namespace, object,
		func(pod corev1.Pod) ([]*runtimepb.ContainerReference, bool) {
			if pod.Status.Phase != corev1.PodRunning && pod.Status.Phase != corev1.PodFailed && pod.Status.Phase != corev1.PodSucceeded {
				return nil, false
			}

			var refs []*runtimepb.ContainerReference

			for _, init := range pod.Status.InitContainerStatuses {
				refs = append(refs, kubedef.MakePodRef(pod.Namespace, pod.Name, init.Name, object))
			}
			for _, container := range pod.Status.ContainerStatuses {
				refs = append(refs, kubedef.MakePodRef(pod.Namespace, pod.Name, container.Name, object))
			}

			return refs, true
		})
}

func (r *ClusterNamespace) resolvePod(ctx context.Context, cli *kubernetes.Clientset, w io.Writer, obj runtime.Deployable) (corev1.Pod, error) {
	return resolvePodByLabels(ctx, cli, w, r.target.namespace, kubedef.SelectById(obj))
}

func (r *ClusterNamespace) DeployedConfigImageID(ctx context.Context, server runtime.Deployable) (oci.ImageID, error) {
	return tasks.Return(ctx, tasks.Action("kubernetes.resolve-config-image-id").Scope(schema.PackageName(server.GetPackageName())),
		func(ctx context.Context) (oci.ImageID, error) {
			// XXX need a StatefulSet variant.
			d, err := r.cluster.cli.AppsV1().Deployments(r.target.namespace).Get(ctx, kubedef.MakeDeploymentId(server), metav1.GetOptions{})
			if err != nil {
				// XXX better error messages.
				return oci.ImageID{}, err
			}

			cfgimage, ok := d.Annotations[kubedef.K8sConfigImage]
			if !ok {
				return oci.ImageID{}, fnerrors.BadInputError("%s: %q is missing as an annotation in %q",
					server.GetPackageName(), kubedef.K8sConfigImage, kubedef.MakeDeploymentId(server))
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

	return r.startTerminal(ctx, r.cluster.cli, server, rio, cmd)
}

func (r *ClusterNamespace) DeleteRecursively(ctx context.Context, wait bool) (bool, error) {
	return DeleteAllRecursively(ctx, r.cluster.cli, wait, nil, r.target.namespace)
}

func (r *ClusterNamespace) DeleteDeployable(ctx context.Context, deployable runtime.Deployable) error {
	listOpts := metav1.ListOptions{LabelSelector: kubedef.SerializeSelector(kubedef.SelectById(deployable))}

	switch deployable.GetDeployableClass() {
	case string(schema.DeployableClass_ONESHOT), string(schema.DeployableClass_MANUAL):
		return r.cluster.cli.CoreV1().Pods(r.target.namespace).DeleteCollection(ctx, metav1.DeleteOptions{}, listOpts)

	case string(schema.DeployableClass_STATEFUL):
		return r.cluster.cli.AppsV1().StatefulSets(r.target.namespace).DeleteCollection(ctx, metav1.DeleteOptions{}, listOpts)

	case string(schema.DeployableClass_STATELESS):
		return r.cluster.cli.AppsV1().Deployments(r.target.namespace).DeleteCollection(ctx, metav1.DeleteOptions{}, listOpts)

	default:
		return fnerrors.InternalError("%s: unsupported deployable class", deployable.GetDeployableClass())
	}
}
