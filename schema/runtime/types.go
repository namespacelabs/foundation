// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package runtime

import (
	"fmt"
	"strings"
)

func (d *Diagnostics) Failed() bool {
	return d.State == Diagnostics_TERMINATED && d.ExitCode > 0
}

func (cw *ContainerWaitStatus) WaitStatus() string {
	var inits []string
	for _, init := range cw.Initializers {
		inits = append(inits, fmt.Sprintf("%s: %s", init.Name, containerStateLabel(init)))
	}

	joinedInits := strings.Join(inits, "; ")

	switch len(cw.Containers) {
	case 0:
		return joinedInits
	case 1:
		return box(containerStateLabel(cw.Containers[0]), joinedInits)
	default:
		var labels []string
		for _, ctr := range cw.Containers {
			labels = append(labels, fmt.Sprintf("%s: %s", ctr.Name, containerStateLabel(ctr)))
		}

		return box(fmt.Sprintf("{%s}", strings.Join(labels, "; ")), joinedInits)
	}
}

func containerStateLabel(st *ContainerUnitWaitStatus) string {
	switch st.Status.GetState() {
	case Diagnostics_RUNNING:
		label := "Running"
		if !st.Status.GetIsReady() {
			label += " (not ready)"
		}
		return label

	case Diagnostics_WAITING:
		return st.Status.GetWaitingReason()

	case Diagnostics_TERMINATED:
		if st.Status.GetExitCode() == 0 {
			return ""
		}

		return fmt.Sprintf("Terminated: %s (exit code %d)", st.Status.GetTerminatedReason(), st.Status.GetExitCode())
	}

	return "(Unknown)"
}

func box(a, b string) string {
	if b == "" {
		return a
	}

	return fmt.Sprintf("%s [%s]", a, b)
}
