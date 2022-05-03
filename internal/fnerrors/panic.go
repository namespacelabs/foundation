// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package fnerrors

import (
	"github.com/pkg/errors"
)

// Re-panics a string as a structured error with a full stack trace.
// `error`s from "github.com/pkg/errors" are trivially passed through
// and other error types are re-panicked after wrapping with a stack trace.
func Panic(v any) {
	// For strings, we re-panic with an error and captured
	// stack frames at the point of invocation below.
	if errmsg, ok := v.(string); ok {
		panic(errors.New(errmsg))
	}
	// Pass through "github.com/pkg/errors" which have stack traces.
	if pkgerr, ok := v.(errors.StackTrace); ok {
		panic(pkgerr)
	}
	// For all other other error types, embellish with stack frames
	// at this point of invocation.
	if err, ok := v.(error); ok {
		panic(errors.WithStack(err))
	}
	// Passthrough for other interface types.
	panic(v)
}
