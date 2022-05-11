// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package fnerrors

import (
	"errors"
	"fmt"
	"io"

	cueerrors "cuelang.org/go/cue/errors"
	"github.com/kr/text"
	"github.com/morikuni/aec"
	"namespacelabs.dev/foundation/internal/fnerrors/stacktrace"
)

// New returns a new error wrapping the given error message with the stack trace
// at the point of invocation.
func New(errmsg string) error {
	return &fnError{Err: errors.New(errmsg), stack: stacktrace.New()}
}

func Wrap(loc Location, err error) error {
	if userErr, ok := err.(*userError); ok {
		if userErr.Location == nil {
			return &userError{fnError: fnError{Err: err, stack: stacktrace.New()}, Location: loc}
		} else if userErr.Location == loc {
			return userErr
		}
	}

	return &userError{fnError: fnError{Err: err, stack: stacktrace.New()}, Location: loc}
}

func Wrapf(loc Location, err error, whatFmt string, args ...interface{}) error {
	args = append(args, err)
	return &userError{fnError: fnError{Err: fmt.Errorf(whatFmt+": %w", args...), stack: stacktrace.New()}, Location: loc}
}

func UserError(loc Location, format string, args ...interface{}) error {
	return &userError{fnError: fnError{Err: fmt.Errorf(format, args...), stack: stacktrace.New()}, Location: loc}
}

// Configuration or system setup is not correct and requires user intervention.
func UsageError(what, whyFmt string, args ...interface{}) error {
	return &usageError{fnError: fnError{Err: fmt.Errorf(whyFmt, args...), stack: stacktrace.New()}, Why: fmt.Sprintf(whyFmt, args...), What: what}
}

// Unexpected situation.
func InternalError(format string, args ...interface{}) error {
	return &internalError{fnError: fnError{Err: fmt.Errorf(format, args...), stack: stacktrace.New()}, expected: false}
}

// A call to a remote endpoint failed, perhaps due to a transient issue.
func InvocationError(format string, args ...interface{}) error {
	return &invocationError{fnError: fnError{Err: fmt.Errorf(format, args...), stack: stacktrace.New()}, expected: false}
}

// The input does match our expectations (e.g. missing bits, wrong version, etc).
func BadInputError(format string, args ...interface{}) error {
	return &internalError{fnError: fnError{Err: fmt.Errorf(format, args...), stack: stacktrace.New()}, expected: false}
}

// We failed but it may be due a transient issue.
func TransientError(format string, args ...interface{}) error {
	return &internalError{fnError: fnError{Err: fmt.Errorf(format, args...), stack: stacktrace.New()}, expected: false}
}

// This error is expected, e.g. a rebuild is required.
func ExpectedError(format string, args ...interface{}) error {
	return &internalError{fnError: fnError{Err: fmt.Errorf(format, args...), stack: stacktrace.New()}, expected: false}
}

// This error means that Foundation does not meet the minimum version requirements.
func DoesNotMeetVersionRequirements(pkg string, expected, got int32) error {
	return &VersionError{pkg, expected, got}
}

// This error is purely for wiring and ensures that Foundation exits with an appropriate exit code.
// The error content has to be output independently.
func ExitWithCode(err error, code int) error {
	return &exitError{fnError: fnError{Err: err, stack: stacktrace.New()}, code: code}
}

// Wraps an error with a stack trace at the point of invocation.
type fnError struct {
	Err   error
	stack stacktrace.StackTrace
}

func (f *fnError) Error() string {
	return f.Err.Error()
}

// Signature is compatible with pkg/errors and allows frameworks like Sentry to
// automatically extract the frame.
func (f *fnError) GetStackTrace() stacktrace.StackTrace {
	return f.stack
}

type userError struct {
	fnError
	Location Location
}

type usageError struct {
	fnError
	Why  string
	What string
}

type internalError struct {
	fnError
	expected bool
}

type invocationError struct {
	fnError
	expected bool
}

func IsExpected(err error) (string, bool) {
	if x, ok := unwrap(err).(*internalError); ok && x.expected {
		return x.Err.Error(), true
	}
	return "", false
}

func (e *userError) Error() string {
	var locStr string
	if e.Location != nil {
		locStr = e.Location.ErrorLocation() + ": "
	}

	return fmt.Sprintf("%s%v", locStr, e.Err)
}

func (e *userError) Unwrap() error { return e.Err }

func (e *usageError) Error() string {
	return fmt.Sprintf("%s\n\n  %s", e.Why, e.What)
}

func (e *internalError) Error() string {
	return e.Err.Error()
}

func (e *invocationError) Error() string {
	return fmt.Sprintf("failed when calling resource: %s", e.Err.Error())
}

type VersionError struct {
	Pkg           string
	Expected, Got int32
}

func (e *VersionError) Error() string {
	return fmt.Sprintf("`fn` needs to be updated to use %q, (need api version %d, got %d)", e.Pkg, e.Expected, e.Got)
}

type ExitError interface {
	ExitCode() int
}

type exitError struct {
	fnError
	Err  error
	code int
}

func (e *exitError) Error() string {
	return e.Err.Error()
}

func (e *exitError) ExitCode() int {
	return e.code
}

func Format(w io.Writer, colors bool, err error) {
	if colors {
		fmt.Fprint(w, aec.RedF.With(aec.Bold).Apply("Failed:\n"))
	} else {
		fmt.Fprint(w, "Failed:\n")
	}
	format(indent(w), colors, err)
}

func format(w io.Writer, colors bool, err error) {
	if x, ok := unwrap(err).(*usageError); ok {
		// XXX don't wordwrap if terminal is below 80 chars in width.
		fmt.Fprintln(w, text.Wrap(x.Why, 80))
		fmt.Fprintln(w)
		fmt.Fprintln(w, "  ", x.What)
		return
	}

	switch x := err.(type) {
	case *userError:
		child := x

		if child.Location == nil {
			if unwrap := errors.Unwrap(child.Err); unwrap == nil {
				fmt.Fprintf(w, " %s\n", child.Err.Error())
			} else {
				fmt.Fprintf(w, "%s:", child.Err.Error())
				fmt.Fprintln(w)
				format(indent(w), colors, unwrap)
			}
		} else {
			for {
				if uec, ok := child.Err.(*userError); ok {
					if uec.Location.ErrorLocation() != x.Location.ErrorLocation() || uec.Err == nil {
						break
					} else {
						child = uec
					}
				} else {
					break
				}
			}

			loc := formatLabel(child.Location.ErrorLocation(), colors)

			// Add the first frame in the stack.
			if len(child.stack) > 0 {
				frame := child.stack[0]
				loc += formatPos(fmt.Sprintf(" (%s:%d)", frame.File(), frame.Line()), colors)
			}
			fmt.Fprintf(w, "%s at %s:", child.Err.Error(), loc)
			fmt.Fprintln(w)
			format(indent(w), colors, child.Err)
		}

	case *internalError:
		fmt.Fprintf(w, "%s: %s\n", bold("internal error", colors), x.Err.Error())
		fmt.Fprintln(w)
		fmt.Fprintf(w, "This was unexpected, please file a bug at https://github.com/namespacelabs/foundation/issues\n")
		errorReportRequest(w)

	case *invocationError:
		fmt.Fprintf(w, "%s: %s\n", bold("invocation error", colors), x.Err.Error())
		fmt.Fprintln(w)
		fmt.Fprintf(w, "This was unexpected, but could be transient. Please try again.\nAnd if it persists, please file a bug at https://github.com/namespacelabs/foundation/issues\n")
		errorReportRequest(w)

	case cueerrors.Error:
		err := cueerrors.Sanitize(x)
		for _, e := range cueerrors.Errors(err) {
			positions := cueerrors.Positions(e)
			if len(positions) == 0 {
				fmt.Fprintln(w, e.Error())
			} else {
				for _, p := range positions {
					pos := p.Position()

					fmt.Fprintln(w, e.Error(), formatPos(pos.String(), colors))
				}
			}
		}

	case *DependencyFailedError:
		fmt.Fprintf(w, "failed to compute %s %s:\n", formatLabel(x.Name, colors), aec.LightBlackF.Apply(fmt.Sprintf("(%s)", x.Type)))
		format(indent(w), colors, x.Err)

	default:
		if unwrapped := errors.Unwrap(x); unwrapped != nil {
			format(indent(w), colors, unwrapped)
		} else {
			fmt.Fprintln(w, x)
		}
	}
}

func errorReportRequest(w io.Writer) {
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Please include,\n")
	fmt.Fprintf(w, "- the full command line you've used.\n")
	fmt.Fprintf(w, "- the full output that fn produced\n")
	fmt.Fprintf(w, "- the output of `fn version`\n")
}

func formatLabel(str string, colors bool) string {
	if colors {
		return aec.CyanF.Apply(str)
	}

	return str
}

func bold(str string, colors bool) string {
	if colors {
		return aec.Bold.Apply(str)
	}
	return str
}

func formatPos(pos string, colors bool) string {
	if colors {
		return aec.LightBlackF.Apply(pos)
	}
	return pos
}

func indent(w io.Writer) io.Writer { return text.NewIndentWriter(w, []byte("  ")) }

func unwrap(err error) error {
	if x := errors.Unwrap(err); x != nil {
		return x
	}
	return err
}
