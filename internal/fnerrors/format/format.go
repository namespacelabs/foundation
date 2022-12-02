// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

// Pretty-printing of various namespace errors.
// Needs to be in a separate package to access error types from elsewhere in the codebase
// without introducing import cycles (e.g. tasks -> fnerrors -> tasks).
package format

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"

	cueerrors "cuelang.org/go/cue/errors"
	"github.com/kr/text"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/console/consolesink"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/std/tasks"
)

type FormatOptions struct {
	// true to use ANSI style.
	style colors.Style
	// If true, we show the chain of foundation errors
	// leading to the root cause.
	tracing bool
	// If true, we show the chain of actions leading to the failed action.
	actionTracing bool
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

func WithActionTrace(tracing bool) FormatOption {
	return func(opts *FormatOptions) {
		opts.actionTracing = tracing
	}
}

func Format(w io.Writer, err error, args ...FormatOption) {
	opts := &FormatOptions{style: colors.NoColors, tracing: false}
	for _, opt := range args {
		opt(opts)
	}
	fmt.Fprint(w, opts.style.ErrorHeader.Apply("Failed: "))

	if opts.tracing {
		fmt.Fprintln(w)
		w = indent(w)
	}

	var actionError *fnerrors.ActionError
	cause := err
	// Keep unwrapping to get the root fnError.
	for {
		// Keep looking for the innermost fnerror
		errors.As(cause, &actionError)

		child := errors.Unwrap(cause)
		if child == nil || !fnerrors.IsNamespaceError(child) {
			break
		} else if opts.tracing {
			format(w, cause, opts)
			writeSourceFileAndLine(w, cause, opts.style)
			w = indent(w)
		}
		cause = child
	}

	if opts.actionTracing && actionError != nil {
		// Print the sequence of actions/tasks leading to the error.
		if !opts.tracing {
			fmt.Fprintln(w) // Break the line after Failed:
			w = indent(w)
		}
		trace := actionError.Trace()
		for i := len(trace) - 1; i >= 0; i-- {
			consolesink.LogAction(w, opts.style, tasks.EventDataFromProto("", trace[i]))
			w = indent(w)
		}
	}

	format(w, cause, opts)
}

func writeSourceFileAndLine(w io.Writer, err error, colors colors.Style) {
	if st, ok := err.(fnerrors.StackTracer); ok {
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
	case *fnerrors.UsageErr:
		formatUsageError(w, x, opts)

	case *fnerrors.BaseError:
		formatUserError(w, x, opts)

	case *fnerrors.InvocationErr:
		formatInvocationError(w, &x.BaseError, opts)

	case *fnerrors.ErrWithLogs:
		formatErrWithLogs(w, x, opts)

	case cueerrors.Error:
		formatCueError(w, x, opts)

	case *fnerrors.DependencyFailedError:
		formatDependencyFailedError(w, x, opts)

	case *fnerrors.CodegenError:
		formatCodegenError(w, opts, x.Error(), x.What, x.PackageName)

	case *fnerrors.CodegenMultiError:
		formatCodegenMultiError(w, x, opts)

	default:
		fmt.Fprintf(w, "%s\n", x.Error())
	}
}

func formatErrWithLogs(w io.Writer, err *fnerrors.ErrWithLogs, opts *FormatOptions) {
	colors := opts.style
	fmt.Fprintf(w, "%s\n", colors.LogCategory.Apply("Captured logs: "))

	const limitLines = 10
	lines := make([]string, 0, limitLines)
	scanner := bufio.NewScanner(err.ReaderF())
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

func formatUsageError(w io.Writer, err *fnerrors.UsageErr, opts *FormatOptions) {
	// XXX don't wordwrap if terminal is below 80 chars in width.
	errTxt := text.Wrap(err.Why, 80)
	fmt.Fprintf(w, "%s\n\n  %s\n", errTxt, opts.style.Highlight.Apply(err.What))
}

func formatInvocationError(w io.Writer, err *fnerrors.BaseError, opts *FormatOptions) {
	fmt.Fprintf(w, "%s: %s\n", opts.style.LogResult.Apply("invocation error"), err.OriginalErr.Error())
	fmt.Fprintln(w)
	fmt.Fprintf(w, "This was unexpected, but could be transient. Please try again.\nAnd if it persists, please run `ns doctor` and file a bug at https://github.com/namespacelabs/foundation/issues\n")
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

func formatDependencyFailedError(w io.Writer, err *fnerrors.DependencyFailedError, opts *FormatOptions) {
	depName := opts.style.LogResult.Apply(err.Name)
	depType := opts.style.LogError.Apply(fmt.Sprintf("(%s)", err.Type))

	if opts.tracing {
		fmt.Fprintf(w, "failed to compute %s %s\n", depName, depType)
	} else {
		fmt.Fprintf(w, "failed to compute %s %s: %s\n", depName, depType, err.OriginalErr)
	}
}

func formatUserError(w io.Writer, err *fnerrors.BaseError, opts *FormatOptions) {
	switch err.Kind {
	case fnerrors.Kind_INTERNAL, fnerrors.Kind_BADINPUT, fnerrors.Kind_BADDATA:
		fmt.Fprintf(w, "%s: %s\n", opts.style.LogResult.Apply("internal error"), err.OriginalErr.Error())
		fmt.Fprintln(w)
		fmt.Fprintf(w, "This was unexpected, please run `ns doctor` and file a bug at https://github.com/namespacelabs/foundation/issues\n")
		errorReportRequest(w)

	case fnerrors.Kind_EXTERNAL:
		fmt.Fprintf(w, "%s: %s\n", opts.style.LogResult.Apply("component error"), err.OriginalErr.Error())
		fmt.Fprintln(w)
		fmt.Fprintf(w, "This was unexpected, please run `ns doctor` and file a bug at https://github.com/namespacelabs/foundation/issues\n")
		errorReportRequest(w)

	case fnerrors.Kind_TRANSIENT:
		formatInvocationError(w, err, opts)

	case fnerrors.Kind_USER:
	default:
		if err.Location != nil {
			loc := opts.style.LogResult.Apply(err.Location.ErrorLocation())
			fmt.Fprintf(w, "%s at %s\n", err.OriginalErr.Error(), loc)
		} else {
			fmt.Fprintf(w, "%s\n", err.OriginalErr.Error())
		}
	}
}

func formatCodegenError(w io.Writer, opts *FormatOptions, err, what string, pkgnames ...string) {
	phase := opts.style.LessRelevant.Apply(what)
	pkgnamesdisplay := opts.style.LogScope.Apply(strings.Join(pkgnames, ", "))
	fmt.Fprintf(w, "%s during %s, for %s %s\n", err, phase, plural(len(pkgnames), "package", "packages"), pkgnamesdisplay)
}

func formatCodegenMultiError(w io.Writer, err *fnerrors.CodegenMultiError, opts *FormatOptions) {
	// Print aggregated errors.
	for commonErr, whatpkgs := range err.CommonErrs {
		for what, pkgs := range whatpkgs {
			var pkgnames []string
			for p := range pkgs {
				pkgnames = append(pkgnames, p)
			}
			formatCodegenError(w, opts, commonErr, what, pkgnames...)
		}
	}
	for _, generr := range err.UniqGenErrs {
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
	fmt.Fprintf(w, "- the output of `ns doctor`\n")
}

func indent(w io.Writer) io.Writer { return text.NewIndentWriter(w, []byte("  ")) }
