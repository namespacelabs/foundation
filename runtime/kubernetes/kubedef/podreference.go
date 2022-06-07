// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubedef

import (
	"fmt"
	"strings"

	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
)

type ContainerPodReference struct {
	Namespace string
	PodName   string
	Container string

	kind runtime.ContainerKind
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

func (cpr ContainerPodReference) Kind() runtime.ContainerKind {
	return cpr.kind
}

func MakePodRef(ns, name, containerName string, srv *schema.Server) runtime.ContainerReference {
	return ContainerPodReference{
		Namespace: ns,
		PodName:   name,
		Container: containerName,
		kind:      decideKind(srv, containerName),
	}
}

func decideKind(srv *schema.Server, containerName string) runtime.ContainerKind {
	if srv == nil {
		return runtime.ContainerKind_Unknown
	}
	if ServerCtrName(srv) == containerName {
		return runtime.ContainerKind_Primary
	}
	return runtime.ContainerKind_Secondary
}

func ServerCtrName(server *schema.Server) string {
	return strings.ToLower(server.Name) // k8s doesn't accept uppercase names.
}
