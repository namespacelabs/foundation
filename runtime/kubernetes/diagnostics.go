// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubernetes

import (
	"context"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/runtime"
)

func (r k8sRuntime) FetchDiagnostics(ctx context.Context, reference runtime.ContainerReference) (runtime.Diagnostics, error) {
	opaque, ok := reference.(containerPodReference)
	if !ok {
		return runtime.Diagnostics{}, fnerrors.InternalError("invalid reference")
	}

	pod, err := r.cli.CoreV1().Pods(opaque.Namespace).Get(ctx, opaque.PodName, metav1.GetOptions{})
	if err != nil {
		return runtime.Diagnostics{}, err
	}

	for _, init := range pod.Status.InitContainerStatuses {
		if init.Name == opaque.Container {
			return statusToDiagnostic(init), nil
		}
	}

	for _, ctr := range pod.Status.ContainerStatuses {
		if ctr.Name == opaque.Container {
			return statusToDiagnostic(ctr), nil
		}
	}

	return runtime.Diagnostics{}, fnerrors.UserError(nil, "%s/%s: no such container %q", opaque.Namespace, opaque.PodName, opaque.Container)
}

func statusToDiagnostic(status v1.ContainerStatus) runtime.Diagnostics {
	var diag runtime.Diagnostics

	diag.RestartCount = status.RestartCount

	switch {
	case status.State.Running != nil:
		diag.Running = true
		diag.Started = status.State.Running.StartedAt.Time
	case status.State.Waiting != nil:
		diag.Waiting = true
		diag.WaitingReason = status.State.Waiting.Reason
		diag.Crashed = status.State.Waiting.Reason == "CrashLoopBackOff"
	case status.State.Terminated != nil:
		diag.Terminated = true
		diag.TerminatedReason = status.State.Terminated.Reason
		diag.ExitCode = status.State.Terminated.ExitCode
	}

	return diag
}
