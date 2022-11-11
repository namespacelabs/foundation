// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package fnerrors

import (
	"errors"
	"fmt"

	"namespacelabs.dev/foundation/internal/fnerrors/stacktrace"
)

func DependencyFailed(name, typ string, err error) error {
	return &DependencyFailedError{BaseError: BaseError{OriginalErr: err, stack: stacktrace.New()}, Name: name, Type: typ}
}

type DependencyFailedError struct {
	BaseError
	Name string
	Type string
}

func (d *DependencyFailedError) Error() string {
	return fmt.Sprintf("resolving %s (%s) failed: %v", d.Name, d.Type, d.OriginalErr)
}

func IsDependencyFailed(err error) bool {
	if _, ok := err.(*DependencyFailedError); ok {
		return true
	}
	if unwrapped := errors.Unwrap(err); unwrapped != nil {
		return IsDependencyFailed(unwrapped)
	}
	return false
}
