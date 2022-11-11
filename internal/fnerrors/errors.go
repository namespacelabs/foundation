// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package fnerrors

import (
	"errors"
	"fmt"
	"io"

	"google.golang.org/grpc/status"
	"namespacelabs.dev/foundation/internal/fnerrors/stacktrace"
	"namespacelabs.dev/foundation/schema/tasks"
	"namespacelabs.dev/foundation/std/tasks/protocol"
)

type ErrorKind string

const (
	Kind_USER       ErrorKind = "ns.error.user"
	Kind_INTERNAL   ErrorKind = "ns.error.internal"
	Kind_EXTERNAL   ErrorKind = "ns.error.external"
	Kind_INVOCATION ErrorKind = "ns.error.invocation"
	Kind_BADINPUT   ErrorKind = "ns.error.badinput"
	Kind_BADDATA    ErrorKind = "ns.error.baddata"
	Kind_TRANSIENT  ErrorKind = "ns.error.transient"
)

// New returns a new error for a format specifier and optionals args with the
// stack trace at the point of invocation. These errors are expected to user
// errors, i.e. they are expected errors, due to wrong configuration, etc.
func New(format string, args ...interface{}) error {
	return &BaseError{Kind: Kind_USER, OriginalErr: fmt.Errorf(format, args...), stack: stacktrace.New()}
}

// NewWithLocation returns a new error for a format specifier and optionals args
// with the stack trace at the point of invocation. These errors are expected to
// user errors, i.e. they are expected errors, due to wrong configuration, etc.
func NewWithLocation(loc Location, format string, args ...interface{}) error {
	return &BaseError{Kind: Kind_USER, OriginalErr: fmt.Errorf(format, args...), stack: stacktrace.New(), Location: loc}
}

func AttachLocation(loc Location, err error) error {
	if userErr, ok := err.(*BaseError); ok {
		if userErr.Location == nil {
			return &BaseError{OriginalErr: userErr.OriginalErr, stack: userErr.stack, Location: loc}
		} else if userErr.Location == loc {
			return userErr
		}
	}

	return &BaseError{OriginalErr: err, stack: stacktrace.New(), Location: loc}
}

func WithLogs(err error, readerF func() io.Reader) error {
	return &ErrWithLogs{err, readerF}
}

func makeError(kind ErrorKind, format string, args ...interface{}) *BaseError {
	return &BaseError{Kind: kind, OriginalErr: fmt.Errorf(format, args...), stack: stacktrace.NewWithSkip(2)}
}

// Configuration or system setup is not correct and requires user intervention.
func UsageError(runThis, toFixThis string, args ...interface{}) error {
	err := makeError(Kind_USER, toFixThis, args...)
	return &UsageErr{BaseError: *err, Why: fmt.Sprintf(toFixThis, args...), What: runThis}
}

// Unexpected error.
func InternalError(format string, args ...interface{}) error {
	return makeError(Kind_INTERNAL, format, args...)
}

// Unexpected error produced by a component external to namespace.
func ExternalError(format string, args ...interface{}) error {
	return makeError(Kind_EXTERNAL, format, args...)
}

// A user-provided input does match our expectations (e.g. missing bits, wrong version, etc).
func BadInputError(format string, args ...interface{}) error {
	return makeError(Kind_BADINPUT, format, args...)
}

// The data does match our expectations (e.g. missing bits, wrong version, etc).
func BadDataError(format string, args ...interface{}) error {
	return makeError(Kind_BADDATA, format, args...)
}

// We failed but it may be due a transient issue.
func TransientError(format string, args ...interface{}) error {
	return makeError(Kind_TRANSIENT, format, args...)
}

// A call to a remote endpoint failed, perhaps due to a transient issue.
func InvocationError(what, format string, args ...interface{}) error {
	err := makeError(Kind_INVOCATION, format, args...)
	return &InvocationErr{BaseError: *err, what: what}
}

func NoAccessToLimitedFeature() error {
	return New("this feature is not broadly available yet; please reach out to us at hello@namespacelabs.com to be added to the access list")
}

// This error means that Namespace does not meet the minimum version requirements.
func DoesNotMeetVersionRequirements(what string, expected, got int32) error {
	return &VersionError{what, expected, got}
}

// This error is purely for wiring and ensures that Namespace exits with an appropriate exit code.
// The error content has to be output independently.
func ExitWithCode(err error, code int) error {
	return &exitError{OriginalErr: err, code: code}
}

// Wraps an error with a stack trace at the point of invocation.
type BaseError struct {
	Kind        ErrorKind
	OriginalErr error
	Location    Location

	stack stacktrace.StackTrace
}

func (e *BaseError) Error() string {
	var locStr string
	if e.Location != nil {
		locStr = e.Location.ErrorLocation() + ": "
	}

	return fmt.Sprintf("%s%v", locStr, e.OriginalErr)
}

func (e *BaseError) IsExpectedError() (error, bool) {
	return e, e.Kind == Kind_USER
}

func (e *BaseError) Unwrap() error { return e.OriginalErr }

// Signature is compatible with pkg/errors and allows frameworks like Sentry to
// automatically extract the frame.
func (e *BaseError) StackTrace() stacktrace.StackTrace {
	return e.stack
}

type UsageErr struct {
	BaseError
	Why  string
	What string
}

type InvocationErr struct {
	BaseError
	what string
}

type ErrWithLogs struct {
	Err     error
	ReaderF func() io.Reader // Returns reader with command's stderr output.
}

func IsExpected(err error) (error, bool) {
	if err == nil {
		return nil, false
	}

	if x, ok := err.(interface {
		IsExpectedError() (error, bool)
	}); ok {
		return x.IsExpectedError()
	}

	if unwrappedError := errors.Unwrap(err); unwrappedError != nil {
		return IsExpected(unwrappedError)
	} else {
		return err, false
	}
}

func (e *UsageErr) Error() string {
	return fmt.Sprintf("%s\n\n  %s", e.Why, e.What)
}

func (e *InvocationErr) Error() string {
	return fmt.Sprintf("failed when calling %s: %s", e.what, e.OriginalErr.Error())
}

func (e *ErrWithLogs) Error() string {
	return e.Err.Error()
}

type VersionError struct {
	What          string
	Expected, Got int32
}

func (e *VersionError) Error() string {
	if e.Expected == 0 && e.Got == 0 {
		return fmt.Sprintf("`ns` needs to be updated to use %q", e.What)
	}

	return fmt.Sprintf("`ns` needs to be updated to use %q, (need api version %d, got %d)", e.What, e.Expected, e.Got)
}

type ExitError interface {
	ExitCode() int
}

type exitError struct {
	OriginalErr error
	code        int
}

func (e *exitError) Error() string {
	return e.OriginalErr.Error()
}

func (e *exitError) ExitCode() int {
	return e.code
}

type StackTracer interface {
	StackTrace() stacktrace.StackTrace
}

// Represents an action error alongside the sequence of actions invocations leading to it.
type ActionError struct {
	ActionID    string
	OriginalErr error
	TraceProto  []*protocol.Task
}

func (ae *ActionError) Error() string           { return ae.OriginalErr.Error() }
func (ae *ActionError) Unwrap() error           { return ae.OriginalErr }
func (ae *ActionError) Trace() []*protocol.Task { return ae.TraceProto }

func (ae *ActionError) GRPCStatus() *status.Status {
	st, _ := status.FromError(ae.OriginalErr)
	p, _ := st.WithDetails(&tasks.ErrorDetail_ActionID{ActionId: ae.ActionID})
	return p
}

func IsNamespaceError(err error) bool {
	switch err.(type) {
	case *BaseError, *InvocationErr, *DependencyFailedError, *VersionError, *ActionError:
		return true
	}
	return false
}
