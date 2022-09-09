// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubernetes

import (
	"context"
	"encoding/json"

	"google.golang.org/protobuf/types/known/anypb"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubeobserver"
	"namespacelabs.dev/foundation/schema/storage"
)

func (r Cluster) FetchDiagnostics(ctx context.Context, reference *runtime.ContainerReference) (*runtime.Diagnostics, error) {
	opaque := &kubedef.ContainerPodReference{}
	if err := reference.Opaque.UnmarshalTo(opaque); err != nil {
		return &runtime.Diagnostics{}, fnerrors.InternalError("invalid reference: %w", err)
	}

	pod, err := r.cli.CoreV1().Pods(opaque.Namespace).Get(ctx, opaque.PodName, metav1.GetOptions{})
	if err != nil {
		return &runtime.Diagnostics{}, err
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

	return &runtime.Diagnostics{}, fnerrors.UserError(nil, "%s/%s: no such container %q", opaque.Namespace, opaque.PodName, opaque.Container)
}

func (r ClusterNamespace) FetchEnvironmentDiagnostics(ctx context.Context) (*storage.EnvironmentDiagnostics, error) {
	systemInfo, err := r.SystemInfo(ctx)
	if err != nil {
		return nil, err
	}

	events, err := r.cli.CoreV1().Events(r.namespace).List(ctx, metav1.ListOptions{})
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
