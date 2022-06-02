// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubernetes

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubeobserver"
)

func (r K8sRuntime) FetchDiagnostics(ctx context.Context, reference runtime.ContainerReference) (runtime.Diagnostics, error) {
	opaque, ok := reference.(kubedef.ContainerPodReference)
	if !ok {
		return runtime.Diagnostics{}, fnerrors.InternalError("invalid reference")
	}

	pod, err := r.cli.CoreV1().Pods(opaque.Namespace).Get(ctx, opaque.PodName, metav1.GetOptions{})
	if err != nil {
		return runtime.Diagnostics{}, err
	}

	for _, init := range pod.Status.InitContainerStatuses {
		if init.Name == opaque.Container {
			return kubeobserver.StatusToDiagnostic(init), nil
		}
	}

	for _, ctr := range pod.Status.ContainerStatuses {
		if ctr.Name == opaque.Container {
			return kubeobserver.StatusToDiagnostic(ctr), nil
		}
	}

	return runtime.Diagnostics{}, fnerrors.UserError(nil, "%s/%s: no such container %q", opaque.Namespace, opaque.PodName, opaque.Container)
}
