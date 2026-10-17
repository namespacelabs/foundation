// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package kubernetes

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/encoding/protojson"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"namespacelabs.dev/foundation/framework/kubernetes/kubedef"
	"namespacelabs.dev/foundation/internal/runtime"
	orchclient "namespacelabs.dev/foundation/orchestration/client"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/tasks"
)

// Run against a disposable cluster; this test creates and deletes a namespace.
func TestClusterNamespaceLifecycle(t *testing.T) {
	kubeconfig := os.Getenv("FOUNDATION_KUBERNETES_TEST_CONFIG")
	if kubeconfig == "" {
		t.Skip("set FOUNDATION_KUBERNETES_TEST_CONFIG to a disposable cluster's kubeconfig")
	}
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if !assert.NoError(t, err) {
		return
	}
	cli, err := k8s.NewForConfig(config)
	if !assert.NoError(t, err) {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	ctx = tasks.WithSink(ctx, tasks.NullSink())
	namespace, err := cli.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{GenerateName: "test-lifecycle-"},
	}, metav1.CreateOptions{})
	if !assert.NoError(t, err) {
		return
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		assert.NoError(t, cli.CoreV1().Namespaces().Delete(ctx, namespace.Name, metav1.DeleteOptions{}))
	})
	useOrchestrator := orchclient.UseOrchestrator
	orchclient.UseOrchestrator = false
	t.Cleanup(func() { orchclient.UseOrchestrator = useOrchestrator })
	cluster := &Cluster{cli: cli, FetchSystemInfo: func(ctx context.Context) (*kubedef.SystemInfo, error) {
		return computeSystemInfo(ctx, cli)
	}}
	bound := &ClusterNamespace{
		parent: cluster, underlying: cluster,
		target: BoundNamespace{namespace: namespace.Name, env: &schema.Environment{Purpose: schema.Environment_TESTING}},
	}

	for _, tt := range []struct {
		name    string
		class   schema.DeployableClass
		command string
		sidecar bool
		wantErr string
	}{
		{"server-zero", schema.DeployableClass_STATELESS, "exit 0", false, "exit code 0"},
		{"server-two", schema.DeployableClass_STATELESS, "exit 2", false, "exit code 2"},
		{"server-sidecar", schema.DeployableClass_STATEFUL, "exit 0", true, "exit code 0"},
		{"oneshot", schema.DeployableClass_ONESHOT, "exit 0", false, ""},
		{"manual", schema.DeployableClass_MANUAL, "exit 0", false, ""},
		{"live-server", schema.DeployableClass_STATELESS, "sleep 600", false, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			deployable := runtime.DeployableSpec{Id: tt.name, Class: tt.class, PackageRef: schema.MakePackageSingleRef("example.com/server")}
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: tt.name, Labels: kubedef.SelectById(deployable), Annotations: map[string]string{"credential": "annotation-sentinel"}},
				Spec: corev1.PodSpec{
					RestartPolicy:  corev1.RestartPolicyNever,
					InitContainers: []corev1.Container{{Name: "init", Image: "busybox:1.36", Command: []string{"sh", "-c", "exit 0"}}},
					Containers: []corev1.Container{{Name: "server", Image: "busybox:1.36", Command: []string{"sh", "-c", tt.command},
						Env: []corev1.EnvVar{{Name: "SECRET", Value: "environment-sentinel"}}}},
				},
			}
			if tt.sidecar {
				pod.Spec.Containers = append(pod.Spec.Containers, corev1.Container{Name: "sidecar", Image: "busybox:1.36", Command: []string{"sleep", "600"}})
			}
			pod, err = cli.CoreV1().Pods(namespace.Name).Create(ctx, pod, metav1.CreateOptions{})
			if !assert.NoError(t, err) {
				return
			}
			err = wait.PollUntilContextTimeout(ctx, 100*time.Millisecond, 90*time.Second, true, func(ctx context.Context) (bool, error) {
				pod, err = cli.CoreV1().Pods(namespace.Name).Get(ctx, tt.name, metav1.GetOptions{})
				if err != nil {
					return false, err
				}
				for _, container := range pod.Status.ContainerStatuses {
					if container.Name == "server" {
						if tt.name == "live-server" {
							return container.Ready, nil
						}
						return container.State.Terminated != nil, nil
					}
				}
				return false, nil
			})
			if !assert.NoError(t, err) {
				return
			}
			readyCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			err = bound.WaitUntilReady(readyCtx, deployable)
			if tt.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tt.wantErr)
				assert.ErrorContains(t, err, "terminated before readiness")
				var failed runtime.ErrContainerFailed
				assert.ErrorAs(t, err, &failed)
			}
			if tt.class == schema.DeployableClass_ONESHOT || tt.class == schema.DeployableClass_MANUAL {
				statuses, err := bound.WaitForTermination(readyCtx, deployable)
				assert.NoError(t, err)
				assert.Len(t, statuses, 2)
				for _, status := range statuses {
					assert.NoError(t, status.TerminationError)
				}
			}
		})
	}

	diagnostics, err := bound.FetchEnvironmentDiagnostics(ctx)
	if !assert.NoError(t, err) {
		return
	}
	output, err := (protojson.MarshalOptions{UseProtoNames: true}).Marshal(diagnostics)
	if !assert.NoError(t, err) {
		return
	}
	assert.NotContains(t, string(output), "environment-sentinel")
	assert.NotContains(t, string(output), "annotation-sentinel")
	var decoded struct {
		RuntimeSpecific []struct {
			Events string `json:"serialized_event_list"`
			Pods   string `json:"serialized_pod_list"`
		} `json:"runtime_specific"`
	}
	if !assert.NoError(t, json.Unmarshal(output, &decoded)) || !assert.Len(t, decoded.RuntimeSpecific, 1) {
		return
	}
	var events corev1.EventList
	assert.NoError(t, json.Unmarshal([]byte(decoded.RuntimeSpecific[0].Events), &events))
	assert.NotEmpty(t, events.Items)
	var pods corev1.PodList
	if !assert.NoError(t, json.Unmarshal([]byte(decoded.RuntimeSpecific[0].Pods), &pods)) {
		return
	}
	assert.Len(t, pods.Items, 6)
	foundExitTwo := false
	for _, pod := range pods.Items {
		assert.Equal(t, namespace.Name, pod.Namespace)
		assert.NotEmpty(t, pod.UID)
		assert.Empty(t, pod.Annotations)
		assert.Empty(t, pod.Spec.Containers)
		if pod.Name == "server-two" {
			foundExitTwo = true
			if assert.Len(t, pod.Status.ContainerStatuses, 1) && assert.NotNil(t, pod.Status.ContainerStatuses[0].State.Terminated) {
				assert.EqualValues(t, 2, pod.Status.ContainerStatuses[0].State.Terminated.ExitCode)
			}
		}
	}
	assert.True(t, foundExitTwo, "missing server-two pod diagnostics")
}
