// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package kubeobserver

import (
	"google.golang.org/protobuf/types/known/timestamppb"
	v1 "k8s.io/api/core/v1"
	"namespacelabs.dev/foundation/schema/runtime"
)

func StatusToDiagnostic(status v1.ContainerStatus) *runtime.Diagnostics {
	diag := &runtime.Diagnostics{}

	diag.RestartCount = status.RestartCount

	switch {
	case status.State.Running != nil:
		diag.Running = true
		diag.Started = timestamppb.New(status.State.Running.StartedAt.Time)
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
