// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubedef

import (
	"fmt"

	"namespacelabs.dev/foundation/runtime"
)

type ContainerPodReference struct {
	Namespace string
	PodName   string
	Container string
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

func MakePodRef(ns, name, containerName string) runtime.ContainerReference {
	return ContainerPodReference{
		Namespace: ns,
		PodName:   name,
		Container: containerName,
	}
}
