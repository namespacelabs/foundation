// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package fnerrors

import (
	"errors"
	"fmt"
	"io"

	"namespacelabs.dev/foundation/internal/fnerrors/stacktrace"
)

// New returns a new error for a format specifier and optionals args with the
// stack trace at the point of invocation.
func New(format string, args ...interface{}) error {
	return &NsError{Err: fmt.Errorf(format, args...), stack: stacktrace.New()}
}

func Wrap(loc Location, err error) error {
	if userErr, ok := err.(*UserErr); ok {
		if userErr.Location == nil {
			return &UserErr{NsError: NsError{Err: userErr.Err, stack: userErr.stack}, Location: loc}
		} else if userErr.Location == loc {
			return userErr
		}
	}

	return &UserErr{NsError: NsError{Err: err, stack: stacktrace.New()}, Location: loc}
}

func Wrapf(loc Location, err error, whatFmt string, args ...interface{}) error {
	args = append(args, err)
	return &UserErr{NsError: NsError{Err: fmt.Errorf(whatFmt+": %w", args...), stack: stacktrace.New()}, Location: loc}
}

func WithLogs(err error, readerF func() io.Reader) error {
	return &ErrWithLogs{err, readerF}
}

func UserError(loc Location, format string, args ...interface{}) error {
	return &UserErr{NsError: NsError{Err: fmt.Errorf(format, args...), stack: stacktrace.New()}, Location: loc}
}

// Configuration or system setup is not correct and requires user intervention.
func UsageError(runThis, toFixThis string, args ...interface{}) error {
	return &UsageErr{NsError: NsError{Err: fmt.Errorf(toFixThis, args...), stack: stacktrace.New()}, Why: fmt.Sprintf(toFixThis, args...), What: runThis}
}

// Unexpected situation.
func InternalError(format string, args ...interface{}) error {
	return &InternalErr{NsError: NsError{Err: fmt.Errorf(format, args...), stack: stacktrace.New()}, expected: false}
}

// A call to a remote endpoint failed, perhaps due to a transient issue.
func InvocationError(format string, args ...interface{}) error {
	return &InvocationErr{NsError: NsError{Err: fmt.Errorf(format, args...), stack: stacktrace.New()}, expected: false}
}

// The input does match our expectations (e.g. missing bits, wrong version, etc).
func BadInputError(format string, args ...interface{}) error {
	return &InternalErr{NsError: NsError{Err: fmt.Errorf(format, args...), stack: stacktrace.New()}, expected: false}
}

// We failed but it may be due a transient issue.
func TransientError(format string, args ...interface{}) error {
	return &InternalErr{NsError: NsError{Err: fmt.Errorf(format, args...), stack: stacktrace.New()}, expected: false}
}

// This error is expected, e.g. a rebuild is required.
func ExpectedError(format string, args ...interface{}) error {
	return &InternalErr{NsError: NsError{Err: fmt.Errorf(format, args...), stack: stacktrace.New()}, expected: true}
}

// This error means that Namespace does not meet the minimum version requirements.
func DoesNotMeetVersionRequirements(what string, expected, got int32) error {
	return &VersionError{what, expected, got}
}

// This error is purely for wiring and ensures that Namespace exits with an appropriate exit code.
// The error content has to be output independently.
func ExitWithCode(err error, code int) error {
	return &exitError{NsError: NsError{Err: err, stack: stacktrace.New()}, code: code}
}

// Wraps an error with a stack trace at the point of invocation.
type NsError struct {
	Err   error
	stack stacktrace.StackTrace
}

func (f *NsError) Error() string {
	return f.Err.Error()
}

func (f *NsError) Unwrap() error { return f.Err }

// Signature is compatible with pkg/errors and allows frameworks like Sentry to
// automatically extract the frame.
func (f *NsError) StackTrace() stacktrace.StackTrace {
	return f.stack
}

type UserErr struct {
	NsError
	Location Location
}

type UsageErr struct {
	NsError
	Why  string
	What string
}

type InternalErr struct {
	NsError
	expected bool
}

type InvocationErr struct {
	NsError
	expected bool
}

type ErrWithLogs struct {
	Err     error
	ReaderF func() io.Reader // Returns reader with command's stderr output.
}

func IsExpected(err error) (string, bool) {
	if x, ok := err.(*InternalErr); ok && x.expected {
		return err.Error(), true
	}
	if _, ok := err.(*UserErr); ok {
		return err.Error(), true
	}
	if x, ok := err.(*CodegenMultiError); ok {
		for _, err := range x.Errs {
			if msg, ok := IsExpected(&err); !ok {
				return msg, false
			}
		}
		return err.Error(), true
	}

	if unwrappedError := errors.Unwrap(err); unwrappedError != nil {
		return IsExpected(unwrappedError)
	} else {
		return err.Error(), false
	}
}

func (e *UserErr) Error() string {
	var locStr string
	if e.Location != nil {
		locStr = e.Location.ErrorLocation() + ": "
	}

	return fmt.Sprintf("%s%v", locStr, e.Err)
}

func (e *UsageErr) Error() string {
	return fmt.Sprintf("%s\n\n  %s", e.Why, e.What)
}

func (e *InternalErr) Error() string {
	return e.Err.Error()
}

func (e *InvocationErr) Error() string {
	return fmt.Sprintf("failed when calling resource: %s", e.Err.Error())
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
	NsError
	code int
}

func (e *exitError) Error() string {
	return e.Err.Error()
}

func (e *exitError) ExitCode() int {
	return e.code
}

type StackTracer interface {
	StackTrace() stacktrace.StackTrace
}
