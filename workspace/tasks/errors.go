// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package tasks

import (
	"context"
	"errors"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/workspace/tasks/protocol"
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

func WrapActionError(err error, culpritID ActionID) *ActionError {
	return &ActionError{err: err, trace: runningActionsSink.Trace(culpritID)}
}

// Represents an action error alongside the sequence of actions invocations leading to it.
type ActionError struct {
	err   error
	trace []*protocol.Task
}

func (ae *ActionError) Error() string           { return ae.err.Error() }
func (ae *ActionError) Unwrap() error           { return ae.err }
func (ae *ActionError) Trace() []*protocol.Task { return ae.trace }
