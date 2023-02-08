// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package kubeobj

import (
	"fmt"

	"namespacelabs.dev/foundation/internal/protos"
	runtimepb "namespacelabs.dev/foundation/schema/runtime"
)

func (cpr *ContainerPodReference) UniqueID() string {
	if cpr.Container == "" {
		return fmt.Sprintf("%s/%s", cpr.Namespace, cpr.PodName)
	}
	return fmt.Sprintf("%s/%s/%s", cpr.Namespace, cpr.PodName, cpr.Container)
}

func MakePodRef(ns, name, containerName string, decideKind func(string) runtimepb.ContainerKind) *runtimepb.ContainerReference {
	cpr := &ContainerPodReference{
		Namespace: ns,
		PodName:   name,
		Container: containerName,
	}

	ref := &runtimepb.ContainerReference{
		UniqueId:       cpr.UniqueID(),
		HumanReference: cpr.Container,
		Opaque:         protos.WrapAnyOrDie(cpr),
	}

	if decideKind != nil {
		ref.Kind = decideKind(containerName)
	}

	return ref
}
