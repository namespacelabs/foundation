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

const errIsCancelled errType = "CANCELLED"
const errIsDependencyFailed errType = "DEPENDENCY FAILED"
const errIsRegular errType = ""

func errorType(err error) errType {
	if errors.Is(err, context.Canceled) {
		return errIsCancelled
	} else if fnerrors.IsDependencyFailed(err) {
		return errIsDependencyFailed
	}

	return errIsRegular
}
