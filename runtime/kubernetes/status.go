// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubernetes

import (
	"fmt"

	v1 "k8s.io/api/core/v1"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/runtime"
)

func waiterFromPodStatus(ns, name string, ps v1.PodStatus) ops.WaitStatus {
	if ps.Phase == v1.PodPending && len(ps.ContainerStatuses) == 0 {
		return pendingWaitStatus{}
	}

	cw := runtime.ContainerWaitStatus{}
	for _, container := range ps.ContainerStatuses {
		if lbl := containerStateLabel(&ps, container.State); lbl != "" {
			cw.Containers = append(cw.Containers, runtime.ContainerUnitWaitStatus{
				Reference:   makePodRef(ns, name, container.Name),
				Name:        container.Name,
				StatusLabel: lbl,
				Status:      statusToDiagnostic(container),
			})
		}
	}

	for _, init := range ps.InitContainerStatuses {
		if lbl := containerStateLabel(nil, init.State); lbl != "" {
			cw.Initializers = append(cw.Initializers, runtime.ContainerUnitWaitStatus{
				Reference:   makePodRef(ns, name, init.Name),
				Name:        init.Name,
				StatusLabel: lbl,
				Status:      statusToDiagnostic(init),
			})
		}
	}

	return cw
}

type pendingWaitStatus struct{}

func (pendingWaitStatus) WaitStatus() string { return "Pending..." }

func containerStateLabel(ps *v1.PodStatus, st v1.ContainerState) string {
	if st.Running != nil {
		label := "Running"
		if ps != nil {
			if !matchPodCondition(*ps, v1.PodReady) {
				label += " (not ready)"
			}
		}
		return label
	}

	if st.Waiting != nil {
		return st.Waiting.Reason
	}

	if st.Terminated != nil {
		if st.Terminated.ExitCode == 0 {
			return ""
		}
		return fmt.Sprintf("Terminated: %s (exit code %d)", st.Terminated.Reason, st.Terminated.ExitCode)
	}

	return "(Unknown)"
}

func makePodRef(ns, name, containerName string) runtime.ContainerReference {
	return containerPodReference{
		Namespace: ns,
		PodName:   name,
		Container: containerName,
	}
}
