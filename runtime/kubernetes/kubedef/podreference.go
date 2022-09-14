// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubedef

import (
	"fmt"
	"strings"

	"namespacelabs.dev/foundation/internal/protos"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
)

func (cpr *ContainerPodReference) UniqueID() string {
	if cpr.Container == "" {
		return fmt.Sprintf("%s/%s", cpr.Namespace, cpr.PodName)
	}
	return fmt.Sprintf("%s/%s/%s", cpr.Namespace, cpr.PodName, cpr.Container)
}

func MakePodRef(ns, name, containerName string, obj runtime.DeployableObject) *runtime.ContainerReference {
	cpr := &ContainerPodReference{
		Namespace: ns,
		PodName:   name,
		Container: containerName,
	}

	return &runtime.ContainerReference{
		UniqueId:       cpr.UniqueID(),
		HumanReference: cpr.Container,
		Kind:           decideKind(obj, containerName),
		Opaque:         protos.WrapAnyOrDie(cpr),
	}
}

func decideKind(obj runtime.DeployableObject, containerName string) schema.ContainerKind {
	if obj == nil {
		return schema.ContainerKind_CONTAINER_KIND_UNSPECIFIED
	}

	if ServerCtrName(obj) == containerName {
		return schema.ContainerKind_PRIMARY
	}

	return schema.ContainerKind_SUPPORT
}

func ServerCtrName(obj runtime.DeployableObject) string {
	return strings.ToLower(obj.GetName()) // k8s doesn't accept uppercase names.
}
