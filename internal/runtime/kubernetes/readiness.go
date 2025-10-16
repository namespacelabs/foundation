// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package kubernetes

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	"namespacelabs.dev/foundation/framework/kubernetes/kubedef"
	"namespacelabs.dev/foundation/framework/kubernetes/kubeobj"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes/kubeobserver"
	"namespacelabs.dev/foundation/std/tasks"
	"namespacelabs.dev/go-ids"
)

type ServiceReadiness struct {
	Ready   bool
	Message string
}

// AreServicesReady checks if all TCP ports of services for a deployable are accepting connections.
// It deploys a one-shot pod in the cluster that attempts TCP connections using in-cluster DNS.
// This works for both in-cluster and remote clusters since we use kubectl to run the pod.
func AreServicesReady(ctx context.Context, cluster *Cluster, namespace string, srv runtime.Deployable) (ServiceReadiness, error) {
	// TODO only check services that are required
	services, err := cluster.cli.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: kubeobj.SerializeSelector(kubedef.SelectById(srv)),
	})
	if err != nil {
		return ServiceReadiness{}, err
	}

	if len(services.Items) == 0 {
		// No services to check
		return ServiceReadiness{Ready: true}, nil
	}

	// Build a shell script that checks all TCP ports with retry logic
	// This runs inside the cluster so it can use service DNS names
	// Uses POSIX-compliant shell syntax for busybox compatibility
	script := "#!/bin/sh\nset -e\n"

	for _, s := range services.Items {
		for _, port := range s.Spec.Ports {
			if port.Protocol != corev1.ProtocolTCP {
				continue
			}

			// Try to connect every 500ms for up to 2 minutes (240 attempts)
			// Use nc (netcat) for TCP connectivity test - available in busybox
			serviceDNS := fmt.Sprintf("%s.%s.svc.cluster.local", s.Name, s.Namespace)
			script += fmt.Sprintf(`
echo "Checking %s:%d..."
i=0
while [ $i -lt 240 ]; do
  if nc -z -w 1 %s %d 2>/dev/null; then
    echo "  ✓ %s:%d is ready"
    break
  fi
  i=$((i + 1))
  if [ $i -eq 240 ]; then
    echo "  ✗ %s:%d failed after 2 minutes"
    exit 1
  fi
  sleep 0.5
done
`, serviceDNS, port.Port, serviceDNS, port.Port, serviceDNS, port.Port, serviceDNS, port.Port)
		}
	}

	script += "echo \"All services ready\"\nexit 0\n"

	// Create a unique pod name for this check
	podName := fmt.Sprintf("ns-svc-check-%s", ids.NewRandomBase32ID(8))

	// Use Chainguard's busybox image for the connection checker
	// This is a minimal, secure base image with shell and basic networking tools
	container := applycorev1.Container().
		WithName("checker").
		WithImage("cgr.dev/chainguard/busybox:latest").
		WithCommand("/bin/sh", "-c", script).
		WithSecurityContext(
			applycorev1.SecurityContext().
				WithReadOnlyRootFilesystem(true).
				WithRunAsNonRoot(true).
				WithRunAsUser(65532)) // nonroot user in Chainguard images

	podSpec := applycorev1.PodSpec().
		WithContainers(container).
		WithRestartPolicy(corev1.RestartPolicyNever).
		WithSecurityContext(applycorev1.PodSecurityContext())

	pod := applycorev1.Pod(podName, namespace).
		WithSpec(podSpec).
		WithLabels(kubedef.SelectNamespaceDriver()).
		WithLabels(kubedef.ManagedByUs())

	// Create the pod
	if _, err := cluster.cli.CoreV1().Pods(namespace).Apply(ctx, pod, kubedef.Ego()); err != nil {
		return ServiceReadiness{}, fmt.Errorf("failed to create service readiness checker pod: %w", err)
	}

	// Schedule cleanup
	defer func() {
		cluster.cli.CoreV1().Pods(namespace).Delete(context.Background(), podName, metav1.DeleteOptions{})
	}()

	// Wait for the pod to complete
	var finalStatus corev1.PodStatus
	if err := kubeobserver.WaitForCondition(ctx, cluster.cli,
		tasks.Action("kubernetes.service-readiness-check").
			Arg("namespace", namespace).
			Arg("deployable", srv.GetName()),
		kubeobserver.WaitForPodConditition(namespace, kubeobserver.PickPod(podName),
			func(status corev1.PodStatus) (bool, error) {
				finalStatus = status
				return status.Phase == corev1.PodSucceeded || status.Phase == corev1.PodFailed, nil
			})); err != nil {
		return ServiceReadiness{}, fmt.Errorf("service readiness check pod failed to complete: %w", err)
	}

	// Check the exit code
	if finalStatus.Phase == corev1.PodSucceeded {
		return ServiceReadiness{Ready: true}, nil
	}

	// Pod failed - extract error message from container status
	var exitCode int32
	var reason string
	for _, containerStatus := range finalStatus.ContainerStatuses {
		if containerStatus.Name == "checker" && containerStatus.State.Terminated != nil {
			exitCode = containerStatus.State.Terminated.ExitCode
			reason = containerStatus.State.Terminated.Reason
			break
		}
	}

	if exitCode == 0 {
		// Should not happen if phase is Failed, but handle it
		return ServiceReadiness{Ready: true}, nil
	}

	return ServiceReadiness{
		Ready:   false,
		Message: fmt.Sprintf("%q not ready: service connectivity check failed (exit code %d, reason: %s)", srv.GetName(), exitCode, reason),
	}, nil
}

// CheckServiceConnectivity is a helper that checks if a specific service port is accepting connections.
// It's used primarily for testing and debugging.
func CheckServiceConnectivity(ctx context.Context, cluster *Cluster, namespace, serviceName string, port int32) error {
	podName := fmt.Sprintf("ns-svc-check-%s", ids.NewRandomBase32ID(8))
	serviceDNS := fmt.Sprintf("%s.%s.svc.cluster.local", serviceName, namespace)

	script := fmt.Sprintf(`#!/bin/sh
i=0
while [ $i -lt 240 ]; do
  if nc -z -w 1 %s %d 2>/dev/null; then
    exit 0
  fi
  i=$((i + 1))
  sleep 0.5
done
exit 1
`, serviceDNS, port)

	container := applycorev1.Container().
		WithName("checker").
		WithImage("cgr.dev/chainguard/busybox:latest").
		WithCommand("/bin/sh", "-c", script).
		WithSecurityContext(
			applycorev1.SecurityContext().
				WithReadOnlyRootFilesystem(true).
				WithRunAsNonRoot(true).
				WithRunAsUser(65532))

	podSpec := applycorev1.PodSpec().
		WithContainers(container).
		WithRestartPolicy(corev1.RestartPolicyNever)

	pod := applycorev1.Pod(podName, namespace).
		WithSpec(podSpec).
		WithLabels(kubedef.ManagedByUs())

	if _, err := cluster.cli.CoreV1().Pods(namespace).Apply(ctx, pod, kubedef.Ego()); err != nil {
		return err
	}

	defer func() {
		cluster.cli.CoreV1().Pods(namespace).Delete(context.Background(), podName, metav1.DeleteOptions{})
	}()

	var finalStatus corev1.PodStatus
	if err := kubeobserver.WaitForCondition(ctx, cluster.cli,
		tasks.Action("kubernetes.check-service-connectivity").
			Arg("namespace", namespace).
			Arg("service", serviceName).
			Arg("port", fmt.Sprintf("%d", port)),
		kubeobserver.WaitForPodConditition(namespace, kubeobserver.PickPod(podName),
			func(status corev1.PodStatus) (bool, error) {
				finalStatus = status
				return status.Phase == corev1.PodSucceeded || status.Phase == corev1.PodFailed, nil
			})); err != nil {
		return err
	}

	if finalStatus.Phase == corev1.PodSucceeded {
		return nil
	}

	return fnerrors.Newf("service %s:%d is not accepting connections", serviceName, port)
}
