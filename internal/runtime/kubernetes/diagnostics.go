// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubernetes

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"namespacelabs.dev/foundation/framework/kubernetes/kubedef"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes/kubeobserver"
	runtimepb "namespacelabs.dev/foundation/schema/runtime"
)

func (r *Cluster) FetchDiagnostics(ctx context.Context, reference *runtimepb.ContainerReference) (*runtimepb.Diagnostics, error) {
	opaque := &kubedef.ContainerPodReference{}
	if err := reference.Opaque.UnmarshalTo(opaque); err != nil {
		return &runtimepb.Diagnostics{}, fnerrors.InternalError("invalid reference: %w", err)
	}

	pod, err := r.cli.CoreV1().Pods(opaque.Namespace).Get(ctx, opaque.PodName, metav1.GetOptions{})
	if err != nil {
		return &runtimepb.Diagnostics{}, err
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

	return &runtimepb.Diagnostics{}, fnerrors.UserError(nil, "%s/%s: no such container %q", opaque.Namespace, opaque.PodName, opaque.Container)
}
