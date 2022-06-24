// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubedef

import (
	"fmt"
	"strings"

	"namespacelabs.dev/foundation/schema"
)

type ContainerPodReference struct {
	Namespace string
	PodName   string
	Container string

	kind schema.ContainerKind
}

func (cpr ContainerPodReference) UniqueID() string {
	if cpr.Container == "" {
		return fmt.Sprintf("%s/%s", cpr.Namespace, cpr.PodName)
	}
	return fmt.Sprintf("%s/%s/%s", cpr.Namespace, cpr.PodName, cpr.Container)
}

func (cpr ContainerPodReference) HumanReference() string {
	return cpr.Container
}

func (cpr ContainerPodReference) Kind() schema.ContainerKind {
	return cpr.kind
}

func MakePodRef(ns, name, containerName string, srv *schema.Server) ContainerPodReference {
	return ContainerPodReference{
		Namespace: ns,
		PodName:   name,
		Container: containerName,
		kind:      decideKind(srv, containerName),
	}
}

func decideKind(srv *schema.Server, containerName string) schema.ContainerKind {
	if srv == nil {
		return schema.ContainerKind_CONTAINER_KIND_UNSPECIFIED
	}
	if ServerCtrName(srv) == containerName {
		return schema.ContainerKind_PRIMARY
	}
	return schema.ContainerKind_SUPPORT
}

func ServerCtrName(server *schema.Server) string {
	return strings.ToLower(server.Name) // k8s doesn't accept uppercase names.
}
