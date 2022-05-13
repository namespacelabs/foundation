// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package fnerrors

import (
	"errors"
	"fmt"

	"namespacelabs.dev/foundation/internal/fnerrors/stacktrace"
)

func DependencyFailed(name, typ string, err error) error {
	return &DependencyFailedError{fnError: fnError{Err: err, stack: stacktrace.New()}, Name: name, Type: typ}
}

type DependencyFailedError struct {
	fnError
	Name string
	Type string
}

func (d *DependencyFailedError) Error() string {
	return fmt.Sprintf("resolving %s (%s) failed: %v", d.Name, d.Type, d.Err)
}
func (d *DependencyFailedError) Unwrap() error { return d.Err }

func IsDependencyFailed(err error) bool {
	if _, ok := err.(*DependencyFailedError); ok {
		return true
	}
	if unwrapped := errors.Unwrap(err); unwrapped != nil {
		return IsDependencyFailed(unwrapped)
	}
	return false
}
