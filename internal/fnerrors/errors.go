// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package fnerrors

import (
	"errors"
	"fmt"
	"io"
	"os/exec"
	"runtime"

	cueerrors "cuelang.org/go/cue/errors"
	"github.com/kr/text"
	"github.com/morikuni/aec"
)

func Wrap(loc Location, err error) error {
	if userErr, ok := err.(*userError); ok {
		if userErr.Location == nil {
			return &userError{Location: loc, Err: userErr.Err, Frame: userErr.Frame}
		} else if userErr.Location == loc {
			return userErr
		}
	}

	return &userError{Location: loc, Err: err, Frame: caller(1)}
}

func Wrapf(loc Location, err error, whatFmt string, args ...interface{}) error {
	args = append(args, err)
	return &userError{loc, fmt.Errorf(whatFmt+": %w", args...), caller(1)}
}

func UserError(loc Location, format string, args ...interface{}) error {
	return &userError{loc, fmt.Errorf(format, args...), caller(1)}
}

// Configuration or system setup is not correct and requires user intervention.
func UsageError(what, whyFmt string, args ...interface{}) error {
	return &usageError{Why: fmt.Sprintf(whyFmt, args...), What: what}
}

// Unexpected situation.
func InternalError(format string, args ...interface{}) error {
	return &internalError{fmt.Errorf(format, args...), false}
}

// A call to a remote endpoint failed, perhaps due to a transient issue.
func InvocationError(format string, args ...interface{}) error {
	return &invocationError{fmt.Errorf(format, args...), false}
}

// The input does match our expectations (e.g. missing bits, wrong version, etc).
func BadInputError(format string, args ...interface{}) error {
	return &internalError{fmt.Errorf(format, args...), false}
}

// We failed but it may be due a transient issue.
func TransientError(format string, args ...interface{}) error {
	return &internalError{fmt.Errorf(format, args...), false}
}

// This error is expected, e.g. a rebuild is required.
func ExpectedError(format string, args ...interface{}) error {
	return &internalError{fmt.Errorf(format, args...), true}
}

// This error means that Foundation does not meet the minimum version requirements.
func DoesNotMeetVersionRequirements(pkg string, expected, got int32) error {
	return &VersionError{pkg, expected, got}
}

// This error is purely for wiring and ensures that Foundation exits with an appropriate exit code.
// The error content has to be output independently.
func ExitError(err error, code int) error {
	return &exitError{err, code}
}

type userError struct {
	Location Location
	Err      error
	Frame    frame
}

type usageError struct {
	Why  string
	What string
}

type internalError struct {
	Err      error
	expected bool
}

type invocationError struct {
	Err      error
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

type exitErrorType interface {
	ExitCode() int
}

type exitError struct {
	Err  error
	code int
}

func (e *exitError) Error() string {
	return e.Err.Error()
}

func (e *exitError) ExitCode() int {
	return e.code
}

func GetExitError(err error) (exitErrorType, bool) {
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr, true
	} else if exitErr, ok := err.(*exitError); ok {
		return exitErr, true
	}
	return nil, false
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

			_, file, line := child.Frame.location()

			loc += formatPos(fmt.Sprintf(" (%s:%d)", file, line), colors)

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

// Adapted from xerror's codebase.
type frame struct {
	frames [3]uintptr
}

func caller(skip int) frame {
	var s frame
	runtime.Callers(skip+1, s.frames[:])
	return s
}

// location reports the file, line, and function of a frame.
//
// The returned function may be "" even if file and line are not.
func (f frame) location() (function, file string, line int) {
	frames := runtime.CallersFrames(f.frames[:])
	if _, ok := frames.Next(); !ok {
		return "", "", 0
	}
	fr, ok := frames.Next()
	if !ok {
		return "", "", 0
	}
	return fr.Function, fr.File, fr.Line
}

func unwrap(err error) error {
	if x := errors.Unwrap(err); x != nil {
		return x
	}
	return err
}
