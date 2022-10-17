// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package tasks

import (
	"context"
	"errors"

	"google.golang.org/grpc/status"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema/tasks"
	"namespacelabs.dev/foundation/std/tasks/protocol"
)

type errType string

const ErrTypeIsCancelled errType = "CANCELLED"
const ErrTypeIsDependencyFailed errType = "DEPENDENCY FAILED"
const ErrTypeIsRegular errType = ""

func ErrorType(err error) errType {
	if errors.Is(err, context.Canceled) {
		return ErrTypeIsCancelled
	} else if fnerrors.IsDependencyFailed(err) {
		return ErrTypeIsDependencyFailed
	}

	return ErrTypeIsRegular
}

func wrapErrorWithAction(err error, actionID ActionID) *ActionError {
	if err == nil {
		return nil
	}

	return &ActionError{err: err, actionID: actionID, trace: runningActionsSink.Trace(actionID)}
}

// Represents an action error alongside the sequence of actions invocations leading to it.
type ActionError struct {
	actionID ActionID
	err      error
	trace    []*protocol.Task
}

func (ae *ActionError) Error() string           { return ae.err.Error() }
func (ae *ActionError) Unwrap() error           { return ae.err }
func (ae *ActionError) Trace() []*protocol.Task { return ae.trace }

func (ae *ActionError) GRPCStatus() *status.Status {
	st, _ := status.FromError(ae.err)
	p, _ := st.WithDetails(&tasks.ErrorDetail_ActionID{ActionId: ae.actionID.String()})
	return p
}
