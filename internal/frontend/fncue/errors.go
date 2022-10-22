// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package fncue

import (
	"fmt"

	"cuelang.org/go/cue/errors"
	"namespacelabs.dev/foundation/framework/rpcerrors/multierr"
)

// Wraps a Cue error to include error location information in the error message.
// Also supports Cue list-errors which are translated into multierr.Error.
func WrapCueError(e error, absPath func(string) string) error {
	cueE, ok := e.(errors.Error)
	if !ok {
		// Pass non-Cue errors as-is.
		return e
	}

	// Deduplicate.
	sanitized := errors.Sanitize(cueE)

	list := errors.Errors(sanitized)
	errs := []error{}
	for _, e := range list {
		errs = append(errs, cueError{cueE: e, absPath: absPath})
	}

	if len(errs) == 1 {
		return errs[0]
	}
	return multierr.New(errs...)
}

type cueError struct {
	cueE    errors.Error
	absPath func(string) string
}

// Implements fnerrors.Location.
func (e cueError) ErrorLocation() string {
	pos := e.cueE.Position()
	if !pos.IsValid() {
		// If we don't have any position information,
		// just pass through the underlying message.
		return e.cueE.Error()
	}

	return fmt.Sprintf("%s:%d:%d", e.absPath(pos.Filename()), pos.Line(), pos.Column())
}

// Implements error.
func (e cueError) Error() string {
	loc := e.ErrorLocation()
	if loc == "" {
		return e.cueE.Error()
	}
	// See also [errors.Print] for examples on how to handle positions.
	return fmt.Sprintf("%s: %s", loc, e.cueE.Error())
}

// Implements error.
func (e cueError) Unwrap() error {
	return e.cueE
}
