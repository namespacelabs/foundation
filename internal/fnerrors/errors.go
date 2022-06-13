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
	"github.com/morikuni/aec"
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
	return &userError{
		fnError:  fnError{Err: err, stack: stacktrace.New()},
		Location: loc,
		What:     fmt.Sprintf(whatFmt, args...),
	}
}

func WithLogs(err error, readerF func() io.Reader) error {
	return &errWithLogs{err, readerF}
}

func UserError(loc Location, format string, args ...interface{}) error {
	return &userError{
		fnError:  fnError{Err: fmt.Errorf(format, args...), stack: stacktrace.New()},
		Location: loc,
	}
}

// Configuration or system setup is not correct and requires user intervention.
func UsageError(what, whyFmt string, args ...interface{}) error {
	return &usageError{
		fnError: fnError{Err: fmt.Errorf(whyFmt, args...), stack: stacktrace.New()},
		Why:     fmt.Sprintf(whyFmt, args...),
		What:    what,
	}
}

// Unexpected situation.
func InternalError(format string, args ...interface{}) error {
	return &internalError{
		fnError:  fnError{Err: fmt.Errorf(format, args...), stack: stacktrace.New()},
		expected: false,
	}
}

// A call to a remote endpoint failed, perhaps due to a transient issue.
func InvocationError(format string, args ...interface{}) error {
	return &invocationError{
		fnError:  fnError{Err: fmt.Errorf(format, args...), stack: stacktrace.New()},
		expected: false,
	}
}

// The input does match our expectations (e.g. missing bits, wrong version, etc).
func BadInputError(format string, args ...interface{}) error {
	return &internalError{
		fnError:  fnError{Err: fmt.Errorf(format, args...), stack: stacktrace.New()},
		expected: false,
	}
}

// We failed but it may be due a transient issue.
func TransientError(format string, args ...interface{}) error {
	return &internalError{
		fnError:  fnError{Err: fmt.Errorf(format, args...), stack: stacktrace.New()},
		expected: false,
	}
}

// This error is expected, e.g. a rebuild is required.
func ExpectedError(format string, args ...interface{}) error {
	return &internalError{
		fnError:  fnError{Err: fmt.Errorf(format, args...), stack: stacktrace.New()},
		expected: true,
	}
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
func (f *fnError) StackTrace() stacktrace.StackTrace {
	return f.stack
}

type userError struct {
	fnError
	What     string
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

type errWithLogs struct {
	Err     error
	readerF func() io.Reader // Returns reader with command's stderr output.
}

func IsExpected(err error) (string, bool) {
	if x, ok := unwrap(err).(*internalError); ok && x.expected {
		return x.Err.Error(), true
	}
	if x, ok := unwrap(err).(*userError); ok {
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

func (e *errWithLogs) Error() string {
	return e.Err.Error()
}

func (e *errWithLogs) Unwrap() error { return e.Err }

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
	code int
}

func (e *exitError) Error() string {
	return e.Err.Error()
}

func (e *exitError) ExitCode() int {
	return e.code
}

type FormatOptions struct {
	// true to use ANSI colors.
	colors bool
	// If true, we show the chain of foundation errors
	// leading to the root cause.
	tracing bool
}

type FormatOption func(*FormatOptions)

func WithColors(colors bool) FormatOption {
	return func(opts *FormatOptions) {
		opts.colors = colors
	}
}

func WithTracing(tracing bool) FormatOption {
	return func(opts *FormatOptions) {
		opts.tracing = tracing
	}
}

func isFnError(err error) bool {
	switch err.(type) {
	case *fnError, *usageError, *userError, *internalError, *invocationError, *DependencyFailedError, *VersionError:
		return true
	}
	return false
}

func Format(w io.Writer, err error, args ...FormatOption) {
	opts := &FormatOptions{colors: false, tracing: false}
	for _, opt := range args {
		opt(opts)
	}
	if opts.colors {
		fmt.Fprint(w, aec.RedF.With(aec.Bold).Apply("Failed: "))
	} else {
		fmt.Fprint(w, "Failed: ")
	}
	if opts.tracing {
		fmt.Fprintln(w)
	}
	cause := err
	// Keep unwrapping to get to the root cause which isn't a fnError.
	for isFnError(cause) {
		if opts.tracing {
			w = indent(w)
			format(w, cause, opts)
			writeSourceFileAndLine(w, cause, opts.colors)
		}
		if x := errors.Unwrap(cause); x != nil {
			cause = x
		} else {
			break
		}
	}
	format(w, cause, opts)
}

func writeSourceFileAndLine(w io.Writer, err error, colors bool) {
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
		if colors {
			fmt.Fprintf(w, "%s\n", aec.LightBlackF.Apply(sourceInfo))
		} else {
			fmt.Fprintf(w, "%s\n", sourceInfo)
		}
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
		formatCodegenError(w, x, opts)

	case *CodegenMultiError:
		formatCodegenMultiError(w, x, opts)

	default:
		fmt.Fprintf(w, "%s\n", x.Error())
	}
}

func formatErrWithLogs(w io.Writer, err *errWithLogs, opts *FormatOptions) {
	colors := opts.colors
	if opts.colors {
		fmt.Fprintf(w, "%s\n", aec.CyanF.With(aec.Bold).Apply("Captured logs: "))
	} else {
		fmt.Fprint(w, "Captured logs: ")
	}
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
		fmt.Fprintf(w, "%s%d%s\n", italic("... (truncated to last ", colors), limitLines, italic(" lines) ...", colors))
	}
	for _, line := range lines {
		fmt.Fprintf(w, "%s\n", line)
	}
	fmt.Fprintln(w)
}

func formatUsageError(w io.Writer, err *usageError, opts *FormatOptions) {
	// XXX don't wordwrap if terminal is below 80 chars in width.
	errTxt := text.Wrap(err.Why, 80)
	fmt.Fprintf(w, "%s: %s %s\n", formatLabel("usage error", opts.colors), errTxt, bold(err.What, opts.colors))
}

func formatInternalError(w io.Writer, err *internalError, opts *FormatOptions) {
	fmt.Fprintf(w, "%s: %s\n", formatLabel("internal error", opts.colors), err.Err.Error())
	fmt.Fprintln(w)
	fmt.Fprintf(w, "This was unexpected, please file a bug at https://github.com/namespacelabs/foundation/issues\n")
	errorReportRequest(w)
}

func formatInvocationError(w io.Writer, err *invocationError, opts *FormatOptions) {
	fmt.Fprintf(w, "%s: %s\n", formatLabel("invocation error", opts.colors), err.Err.Error())
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

				fmt.Fprintln(w, e.Error(), formatPos(pos.String(), opts.colors))
			}
		}
	}
}

func formatDependencyFailedError(w io.Writer, err *DependencyFailedError, opts *FormatOptions) {
	depName := formatLabel(err.Name, opts.colors)

	depType := fmt.Sprintf("(%s)", err.Type)
	if opts.colors {
		depType = aec.LightMagentaF.Apply(depType)
	}

	if opts.tracing {
		fmt.Fprintf(w, "failed to compute %s %s\n", depName, depType)
	} else {
		fmt.Fprintf(w, "failed to compute %s %s: %s\n", depName, depType, err.Err)
	}
}

func formatUserError(w io.Writer, err *userError, opts *FormatOptions) {
	what := err.What
	if len(what) > 0 {
		what = ": " + what
	}
	if err.Location != nil {
		loc := formatLabel(err.Location.ErrorLocation(), opts.colors)
		fmt.Fprintf(w, "%s%s: %s\n", loc, what, err.Err.Error())
	} else {
		fmt.Fprintf(w, "%s%s\n", what, err.Err.Error())
	}
}

func formatCodegenError(w io.Writer, err *CodegenError, opts *FormatOptions) {
	phase := err.What
	if opts.colors {
		phase = aec.MagentaF.Apply(phase)
	}
	pkgName := err.PackageName
	if opts.colors {
		pkgName = aec.LightBlackF.Apply(pkgName)
	}
	fmt.Fprintf(w, "%s at phase [%s] for package %s\n", err.Error(), phase, pkgName)
}

func formatCodegenMultiError(w io.Writer, err *CodegenMultiError, opts *FormatOptions) {
	// Print aggregated errors.
	for commonErr, whatpkgs := range err.commonerrs {
		for what, pkgs := range whatpkgs {
			var pkgnames []string
			for p := range pkgs {
				pkgnames = append(pkgnames, p)
			}
			phase := what
			if opts.colors {
				phase = aec.MagentaF.Apply(phase)
			}
			pkgnamesdisplay := strings.Join(pkgnames, ", ")
			if opts.colors {
				pkgnamesdisplay = aec.LightBlackF.Apply(pkgnamesdisplay)
			}
			fmt.Fprintf(w, "%s at phase [%s] for package(s) %s\n", commonErr.Error(), phase, pkgnamesdisplay)
		}
	}
	for _, generr := range err.uniqgenerrs {
		formatCodegenError(w, &generr, opts)
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

func italic(str string, colors bool) string {
	if colors {
		return aec.Italic.Apply(str)
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
