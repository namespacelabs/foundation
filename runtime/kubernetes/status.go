// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubernetes

import (
	v1 "k8s.io/api/core/v1"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/runtime"
)

func waiterFromPodStatus(ns, name string, ps v1.PodStatus) ops.WaitStatus {
	cw := runtime.ContainerWaitStatus{}
	for _, container := range ps.ContainerStatuses {
		if lbl := containerStateLabel(container.State); lbl != "" {
			cw.Containers = append(cw.Containers, runtime.ContainerUnitWaitStatus{
				Reference: makePodRef(ns, name, container.Name),
				Name:      container.Name,
				Status:    lbl,
			})
		}
	}

	for _, init := range ps.InitContainerStatuses {
		if lbl := containerStateLabel(init.State); lbl != "" {
			cw.Initializers = append(cw.Initializers, runtime.ContainerUnitWaitStatus{
				Reference: makePodRef(ns, name, init.Name),
				Name:      init.Name,
				Status:    lbl,
			})
		}
	}

	return cw
}

func makePodRef(ns, name, containerName string) *runtime.ContainerReference {
	return &runtime.ContainerReference{
		Opaque: containerPodReference{
			Namespace: ns,
			Name:      name,
			Container: containerName,
		},
	}
}
