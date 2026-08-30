// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package tasks

import (
	"context"
	"errors"

	"namespacelabs.dev/foundation/internal/fnerrors"
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

func wrapErrorWithAction(err error, actionID ActionID) *fnerrors.ActionError {
	if err == nil {
		return nil
	}

	return &fnerrors.ActionError{OriginalErr: err, ActionID: actionID.String(), TraceProto: runningActionsSink.Trace(actionID)}
}
