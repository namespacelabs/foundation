// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubeobserver

import (
	v1 "k8s.io/api/core/v1"
	"namespacelabs.dev/foundation/runtime"
)

func StatusToDiagnostic(status v1.ContainerStatus) runtime.Diagnostics {
	var diag runtime.Diagnostics

	diag.RestartCount = status.RestartCount

	switch {
	case status.State.Running != nil:
		diag.Running = true
		diag.Started = status.State.Running.StartedAt.Time
	case status.State.Waiting != nil:
		diag.Waiting = true
		diag.WaitingReason = status.State.Waiting.Reason
		diag.Crashed = status.State.Waiting.Reason == "CrashLoopBackOff"
	case status.State.Terminated != nil:
		diag.Terminated = true
		diag.TerminatedReason = status.State.Terminated.Reason
		diag.ExitCode = status.State.Terminated.ExitCode
	}

	return diag
}
