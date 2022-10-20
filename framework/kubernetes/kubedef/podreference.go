// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubedef

import (
	"fmt"
	"strings"

	"namespacelabs.dev/foundation/internal/protos"
	"namespacelabs.dev/foundation/internal/runtime"
	runtimepb "namespacelabs.dev/foundation/schema/runtime"
)

func (cpr *ContainerPodReference) UniqueID() string {
	if cpr.Container == "" {
		return fmt.Sprintf("%s/%s", cpr.Namespace, cpr.PodName)
	}
	return fmt.Sprintf("%s/%s/%s", cpr.Namespace, cpr.PodName, cpr.Container)
}

func MakePodRef(ns, name, containerName string, obj runtime.Deployable) *runtimepb.ContainerReference {
	cpr := &ContainerPodReference{
		Namespace: ns,
		PodName:   name,
		Container: containerName,
	}

	return &runtimepb.ContainerReference{
		UniqueId:       cpr.UniqueID(),
		HumanReference: cpr.Container,
		Kind:           decideKind(obj, containerName),
		Opaque:         protos.WrapAnyOrDie(cpr),
	}
}

func decideKind(obj runtime.Deployable, containerName string) runtimepb.ContainerKind {
	if obj == nil {
		return runtimepb.ContainerKind_CONTAINER_KIND_UNSPECIFIED
	}

	if ServerCtrName(obj) == containerName {
		return runtimepb.ContainerKind_PRIMARY
	}

	return runtimepb.ContainerKind_SUPPORT
}

func ServerCtrName(obj runtime.Deployable) string {
	return strings.ToLower(obj.GetName()) // k8s doesn't accept uppercase names.
}
