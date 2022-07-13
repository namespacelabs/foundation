// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package fnerrors

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"

	cueerrors "cuelang.org/go/cue/errors"
	"github.com/kr/text"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/fnerrors/stacktrace"
)

// New returns a new error for a format specifier and optionals args with the
// stack trace at the point of invocation.
func New(format string, args ...interface{}) error {
	return &fnError{Err: fmt.Errorf(format, args...), stack: stacktrace.New()}
}

func Wrap(loc Location, err error) error {
	if userErr, ok := err.(*userError); ok {
		if userErr.Location == nil {
			return &userError{fnError: fnError{Err: userErr.Err, stack: userErr.stack}, Location: loc}
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

func WithLogs(err error, readerF func() io.Reader) error {
	return &errWithLogs{err, readerF}
}

func UserError(loc Location, format string, args ...interface{}) error {
	return &userError{fnError: fnError{Err: fmt.Errorf(format, args...), stack: stacktrace.New()}, Location: loc}
}

// Configuration or system setup is not correct and requires user intervention.
func UsageError(runThis, toFixThis string, args ...interface{}) error {
	return &usageError{fnError: fnError{Err: fmt.Errorf(toFixThis, args...), stack: stacktrace.New()}, Why: fmt.Sprintf(toFixThis, args...), What: runThis}
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
	return &internalError{fnError: fnError{Err: fmt.Errorf(format, args...), stack: stacktrace.New()}, expected: true}
}

// This error means that Namespace does not meet the minimum version requirements.
func DoesNotMeetVersionRequirements(pkg string, expected, got int32) error {
	return &VersionError{pkg, expected, got}
}

// This error is purely for wiring and ensures that Namespace exits with an appropriate exit code.
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

func (f *fnError) Unwrap() error { return f.Err }

// Signature is compatible with pkg/errors and allows frameworks like Sentry to
// automatically extract the frame.
func (f *fnError) StackTrace() stacktrace.StackTrace {
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

func IsInvocationError(err error) bool {
	if _, ok := err.(*invocationError); ok {
		return true
	}
	if unwrapped := errors.Unwrap(err); unwrapped != nil {
		return IsInvocationError(unwrapped)
	}
	return false
}

type errWithLogs struct {
	Err     error
	readerF func() io.Reader // Returns reader with command's stderr output.
}

func IsExpected(err error) (string, bool) {
	if x, ok := err.(*internalError); ok && x.expected {
		return err.Error(), true
	}
	if _, ok := err.(*userError); ok {
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

func (e *userError) Error() string {
	var locStr string
	if e.Location != nil {
		locStr = e.Location.ErrorLocation() + ": "
	}

	return fmt.Sprintf("%s%v", locStr, e.Err)
}

func (e *usageError) Error() string {
	return fmt.Sprintf("%s\n\n  %s", e.Why, e.What)
}

func (e *internalError) Error() string {
	return e.Err.Error()
}

func (e *invocationError) Error() string {
	return fmt.Sprintf("failed when calling resource: %s", e.Err.Error())
}

func (e *errWithLogs) Error() string {
	return e.Err.Error()
}

type VersionError struct {
	Pkg           string
	Expected, Got int32
}

func (e *VersionError) Error() string {
	return fmt.Sprintf("`ns` needs to be updated to use %q, (need api version %d, got %d)", e.Pkg, e.Expected, e.Got)
}

type ExitError interface {
	ExitCode() int
}

type exitError struct {
	fnError
	code int
}

func (e *exitError) Error() string {
	return e.Err.Error()
}

func (e *exitError) ExitCode() int {
	return e.code
}

type FormatOptions struct {
	// true to use ANSI style.
	style colors.Style
	// If true, we show the chain of foundation errors
	// leading to the root cause.
	tracing bool
}

type FormatOption func(*FormatOptions)

func WithStyle(style colors.Style) FormatOption {
	return func(opts *FormatOptions) {
		opts.style = style
	}
}

func WithTracing(tracing bool) FormatOption {
	return func(opts *FormatOptions) {
		opts.tracing = tracing
	}
}

func isFnError(err error) bool {
	switch err.(type) {
	case *fnError, *userError, *internalError, *invocationError, *DependencyFailedError, *VersionError:
		return true
	}
	return false
}

func Format(w io.Writer, err error, args ...FormatOption) {
	opts := &FormatOptions{style: colors.NoColors, tracing: false}
	for _, opt := range args {
		opt(opts)
	}
	fmt.Fprint(w, opts.style.ErrorHeader.Apply("Failed: "))

	if opts.tracing {
		fmt.Fprintln(w)
	}
	cause := err
	// Keep unwrapping to get the root fnError.
	for {
		if opts.tracing && cause != err {
			w = indent(w)
			format(w, cause, opts)
			writeSourceFileAndLine(w, cause, opts.style)
		}
		child := errors.Unwrap(cause)
		if child == nil || !isFnError(child) {
			break
		}
		cause = child
	}
	format(w, cause, opts)
}

func writeSourceFileAndLine(w io.Writer, err error, colors colors.Style) {
	type stackTracer interface {
		StackTrace() stacktrace.StackTrace
	}
	if st, ok := err.(stackTracer); ok {
		stack := st.StackTrace()
		if len(stack) == 0 {
			return
		}
		frame := stack[0]
		sourceInfo := fmt.Sprintf("%s:%d", frame.File(), frame.Line())
		fmt.Fprintf(w, "%s\n", colors.Header.Apply(sourceInfo))
	}
}

func format(w io.Writer, err error, opts *FormatOptions) {
	if err == nil {
		return
	}

	switch x := err.(type) {
	case *usageError:
		formatUsageError(w, x, opts)

	case *userError:
		formatUserError(w, x, opts)

	case *internalError:
		formatInternalError(w, x, opts)

	case *invocationError:
		formatInvocationError(w, x, opts)

	case *errWithLogs:
		formatErrWithLogs(w, x, opts)

	case cueerrors.Error:
		formatCueError(w, x, opts)

	case *DependencyFailedError:
		formatDependencyFailedError(w, x, opts)

	case *CodegenError:
		formatCodegenError(w, opts, x.Error(), x.What, x.PackageName)

	case *CodegenMultiError:
		formatCodegenMultiError(w, x, opts)

	default:
		fmt.Fprintf(w, "%s\n", x.Error())
	}
}

func formatErrWithLogs(w io.Writer, err *errWithLogs, opts *FormatOptions) {
	colors := opts.style
	fmt.Fprintf(w, "%s\n", colors.LogCategory.Apply("Captured logs: "))

	const limitLines = 10
	lines := make([]string, 0, limitLines)
	scanner := bufio.NewScanner(err.readerF())
	truncated := false
	for scanner.Scan() {
		if len(lines) < limitLines {
			lines = append(lines, scanner.Text())
		} else {
			truncated = true
			lines = append(lines[1:], scanner.Text())
		}
	}
	if truncated {
		fmt.Fprintf(w, "%s%d%s\n", colors.LessRelevant.Apply("... (truncated to last "), limitLines, colors.LessRelevant.Apply(" lines) ..."))
	}
	for _, line := range lines {
		fmt.Fprintf(w, "%s\n", line)
	}
	fmt.Fprintln(w)
}

func formatUsageError(w io.Writer, err *usageError, opts *FormatOptions) {
	// XXX don't wordwrap if terminal is below 80 chars in width.
	errTxt := text.Wrap(err.Why, 80)
	fmt.Fprintf(w, "%s\n\n  %s\n", errTxt, opts.style.Highlight.Apply(err.What))
}

func formatInternalError(w io.Writer, err *internalError, opts *FormatOptions) {
	fmt.Fprintf(w, "%s: %s\n", opts.style.LogResult.Apply("internal error"), err.Err.Error())
	fmt.Fprintln(w)
	fmt.Fprintf(w, "This was unexpected, please file a bug at https://github.com/namespacelabs/foundation/issues\n")
	errorReportRequest(w)
}

func formatInvocationError(w io.Writer, err *invocationError, opts *FormatOptions) {
	fmt.Fprintf(w, "%s: %s\n", opts.style.LogResult.Apply("invocation error"), err.Err.Error())
	fmt.Fprintln(w)
	fmt.Fprintf(w, "This was unexpected, but could be transient. Please try again.\nAnd if it persists, please file a bug at https://github.com/namespacelabs/foundation/issues\n")
	errorReportRequest(w)
}

func formatCueError(w io.Writer, err cueerrors.Error, opts *FormatOptions) {
	errclean := cueerrors.Sanitize(err)
	for _, e := range cueerrors.Errors(errclean) {
		positions := cueerrors.Positions(e)
		if len(positions) == 0 {
			fmt.Fprintln(w, e.Error())
		} else {
			for _, p := range positions {
				pos := p.Position()

				fmt.Fprintln(w, e.Error(), opts.style.Header.Apply(pos.String()))
			}
		}
	}
}

func formatDependencyFailedError(w io.Writer, err *DependencyFailedError, opts *FormatOptions) {
	depName := opts.style.LogResult.Apply(err.Name)
	depType := opts.style.LogError.Apply(fmt.Sprintf("(%s)", err.Type))

	if opts.tracing {
		fmt.Fprintf(w, "failed to compute %s %s\n", depName, depType)
	} else {
		fmt.Fprintf(w, "failed to compute %s %s: %s\n", depName, depType, err.Err)
	}
}

func formatUserError(w io.Writer, err *userError, opts *FormatOptions) {
	if err.Location != nil {
		loc := opts.style.LogResult.Apply(err.Location.ErrorLocation())
		fmt.Fprintf(w, "%s at %s\n", err.Err.Error(), loc)
	} else {
		fmt.Fprintf(w, "%s\n", err.Err.Error())
	}
}

func formatCodegenError(w io.Writer, opts *FormatOptions, err, what string, pkgnames ...string) {
	phase := opts.style.LessRelevant.Apply(what)
	pkgnamesdisplay := opts.style.LogScope.Apply(strings.Join(pkgnames, ", "))
	fmt.Fprintf(w, "%s during %s, for %s %s\n", err, phase, plural(len(pkgnames), "package", "packages"), pkgnamesdisplay)
}

func formatCodegenMultiError(w io.Writer, err *CodegenMultiError, opts *FormatOptions) {
	// Print aggregated errors.
	for commonErr, whatpkgs := range err.commonerrs {
		for what, pkgs := range whatpkgs {
			var pkgnames []string
			for p := range pkgs {
				pkgnames = append(pkgnames, p)
			}
			formatCodegenError(w, opts, commonErr, what, pkgnames...)
		}
	}
	for _, generr := range err.uniqgenerrs {
		formatCodegenError(w, opts, generr.Error(), generr.What, generr.PackageName)
	}
}

func plural(count int, singular, plural string) string {
	if count == 1 {
		return singular
	}
	return plural
}

func errorReportRequest(w io.Writer) {
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Please include,\n")
	fmt.Fprintf(w, "- the full command line you've used.\n")
	fmt.Fprintf(w, "- the full output that ns produced\n")
	fmt.Fprintf(w, "- the output of `ns version`\n")
}

func indent(w io.Writer) io.Writer { return text.NewIndentWriter(w, []byte("  ")) }
