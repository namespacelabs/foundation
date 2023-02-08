// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package kubeobserver

import (
	v1 "k8s.io/api/core/v1"
	"namespacelabs.dev/foundation/framework/kubernetes/kubeobj"
	"namespacelabs.dev/foundation/internal/protos"
	"namespacelabs.dev/foundation/schema/orchestration"
	"namespacelabs.dev/foundation/schema/runtime"
)

func PodStatusToWaitStatus(ns, name string, ps v1.PodStatus) *orchestration.Event_WaitStatus {
	if ps.Phase == v1.PodPending && len(ps.ContainerStatuses) == 0 {
		return &orchestration.Event_WaitStatus{Description: "Pending..."}
	}

	_, isReady := MatchPodCondition(ps, v1.PodReady)
	cw := &runtime.ContainerWaitStatus{
		IsReady: isReady,
	}

	for _, container := range ps.ContainerStatuses {
		if status := StatusToDiagnostic(container); status != nil {
			cw.Containers = append(cw.Containers, &runtime.ContainerUnitWaitStatus{
				Reference: kubeobj.MakePodRef(ns, name, container.Name, nil),
				Name:      container.Name,
				Status:    status,
			})
		}
	}

	for _, init := range ps.InitContainerStatuses {
		if status := StatusToDiagnostic(init); status != nil {
			cw.Initializers = append(cw.Initializers, &runtime.ContainerUnitWaitStatus{
				Reference: kubeobj.MakePodRef(ns, name, init.Name, nil),
				Name:      init.Name,
				Status:    status,
			})
		}
	}

	return &orchestration.Event_WaitStatus{
		Description: cw.WaitStatus(),
		Opaque:      protos.WrapAnyOrDie(cw),
	}
}
