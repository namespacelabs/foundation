// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

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
