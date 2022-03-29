// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package tasks

import (
	"context"
	"errors"
	"fmt"
)

func DependencyFailed(name, typ string, err error) error {
	return &DependencyFailedError{name, typ, err}
}

type DependencyFailedError struct {
	Name string
	Type string
	Err  error
}

func (d *DependencyFailedError) Error() string {
	return fmt.Sprintf("resolving %s (%s) failed: %v", d.Name, d.Type, d.Err)
}
func (d *DependencyFailedError) Unwrap() error { return d.Err }

func isDependencyFailed(err error) bool {
	if _, ok := err.(*DependencyFailedError); ok {
		return true
	}
	if unwrapped := errors.Unwrap(err); unwrapped != nil {
		return isDependencyFailed(unwrapped)
	}
	return false
}

type errType string

const errIsCancelled errType = "CANCELLED"
const errIsDependencyFailed errType = "DEPENDENCY FAILED"
const errIsRegular errType = ""

func errorType(err error) errType {
	if errors.Is(err, context.Canceled) {
		return errIsCancelled
	} else if isDependencyFailed(err) {
		return errIsDependencyFailed
	}

	return errIsRegular
}