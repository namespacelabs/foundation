// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation
//
// Adapted from Sentry's codebase.

package fnerrors

import (
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
)

const unknown string = "unknown"

// Frame represents a function call and it's metadata. Frames are associated
// with a Stacktrace.
type Frame struct {
	Function string `json:"function,omitempty"`
	Symbol   string `json:"symbol,omitempty"`
	Module   string `json:"module,omitempty"`
	Filename string `json:"filename,omitempty"`
	AbsPath  string `json:"abs_path,omitempty"`
	Lineno   int    `json:"lineno,omitempty"`
}

// Stacktrace holds information about the frames of the stack.
type Stacktrace struct {
	Frames []Frame `json:"frames,omitempty"`
}

// ErrorStacktrace decorates a Stacktrace with an error message.
type ErrorStacktrace struct {
	Errmsg string     `json:"errmsg,omitempty"`
	Trace  Stacktrace `json:"trace"`
}

func NewErrorStacktrace(err error) *ErrorStacktrace {
	method := extractReflectedStacktraceMethod(err)

	var pcs []uintptr

	if method.IsValid() {
		pcs = extractPcs(method)
	} else {
		pcs = extractXErrorsPC(err)
	}

	if len(pcs) == 0 {
		return nil
	}

	frames := extractFrames(pcs)
	frames = filterFrames(frames)

	stacktrace := Stacktrace{
		Frames: frames,
	}
	return &ErrorStacktrace{Trace: stacktrace, Errmsg: err.Error()}
}

func extractReflectedStacktraceMethod(err error) reflect.Value {
	var method reflect.Value

	// https://github.com/pingcap/errors
	methodGetStackTracer := reflect.ValueOf(err).MethodByName("GetStackTracer")
	// https://github.com/pkg/errors
	methodStackTrace := reflect.ValueOf(err).MethodByName("StackTrace")
	// https://github.com/go-errors/errors
	methodStackFrames := reflect.ValueOf(err).MethodByName("StackFrames")

	if methodGetStackTracer.IsValid() {
		stacktracer := methodGetStackTracer.Call(make([]reflect.Value, 0))[0]
		stacktracerStackTrace := reflect.ValueOf(stacktracer).MethodByName("StackTrace")

		if stacktracerStackTrace.IsValid() {
			method = stacktracerStackTrace
		}
	}

	if methodStackTrace.IsValid() {
		method = methodStackTrace
	}

	if methodStackFrames.IsValid() {
		method = methodStackFrames
	}

	return method
}

func extractPcs(method reflect.Value) []uintptr {
	var pcs []uintptr

	stacktrace := method.Call(make([]reflect.Value, 0))[0]

	if stacktrace.Kind() != reflect.Slice {
		return nil
	}

	for i := 0; i < stacktrace.Len(); i++ {
		pc := stacktrace.Index(i)

		switch pc.Kind() {
		case reflect.Uintptr:
			pcs = append(pcs, uintptr(pc.Uint()))
		case reflect.Struct:
			for _, fieldName := range []string{"ProgramCounter", "PC"} {
				field := pc.FieldByName(fieldName)
				if !field.IsValid() {
					continue
				}
				if field.Kind() == reflect.Uintptr {
					pcs = append(pcs, uintptr(field.Uint()))
					break
				}
			}
		}
	}

	return pcs
}

// extractXErrorsPC extracts program counters from error values compatible with
// the error types from golang.org/x/xerrors.
//
// It returns nil if err is not compatible with errors from that package or if
// no program counters are stored in err.
func extractXErrorsPC(err error) []uintptr {
	// This implementation uses the reflect package to avoid a hard dependency
	// on third-party packages.

	// We don't know if err matches the expected type. For simplicity, instead
	// of trying to account for all possible ways things can go wrong, some
	// assumptions are made and if they are violated the code will panic. We
	// recover from any panic and ignore it, returning nil.
	//nolint: errcheck
	defer func() { recover() }()

	field := reflect.ValueOf(err).Elem().FieldByName("frame") // type Frame struct{ frames [3]uintptr }
	field = field.FieldByName("frames")
	field = field.Slice(1, field.Len()) // drop first pc pointing to xerrors.New
	pc := make([]uintptr, field.Len())
	for i := 0; i < field.Len(); i++ {
		pc[i] = uintptr(field.Index(i).Uint())
	}
	return pc
}

// NewFrame assembles a stacktrace frame out of runtime.Frame.
func NewFrame(f runtime.Frame) Frame {
	var abspath, relpath string
	switch {
	case f.File == "":
		relpath = unknown
		// Omit abspath from serialization.
		abspath = ""
	case filepath.IsAbs(f.File):
		abspath = f.File
		// Omit relpath from serialization.
		relpath = ""
	default:
		relpath = f.File
		// Omit abspath from serialization.
		abspath = ""
	}

	function := f.Function
	var pkg string

	if function != "" {
		pkg, function = splitQualifiedFunctionName(function)
	}

	frame := Frame{
		AbsPath:  abspath,
		Filename: relpath,
		Lineno:   f.Line,
		Module:   pkg,
		Function: function,
	}
	return frame
}

// splitQualifiedFunctionName splits a package path-qualified function name into
// package name and function name. Such qualified names are found in
// runtime.Frame.Function values.
func splitQualifiedFunctionName(name string) (pkg string, fun string) {
	pkg = packageName(name)
	fun = strings.TrimPrefix(name, pkg+".")
	return
}

func extractFrames(pcs []uintptr) []Frame {
	var frames []Frame
	callersFrames := runtime.CallersFrames(pcs)

	for {
		callerFrame, more := callersFrames.Next()

		frames = append([]Frame{
			NewFrame(callerFrame),
		}, frames...)

		if !more {
			break
		}
	}

	return frames
}

// filterFrames filters out stack frames that are not meant to be reported
// to foundation such as go internal frames.
func filterFrames(frames []Frame) []Frame {
	if len(frames) == 0 {
		return nil
	}

	filteredFrames := make([]Frame, 0, len(frames))

	for _, frame := range frames {
		// Skip Go internal frames.
		if frame.Module == "runtime" || frame.Module == "testing" {
			continue
		}
		filteredFrames = append(filteredFrames, frame)
	}

	return filteredFrames
}

// packageName returns the package part of the symbol name, or the empty string
// if there is none.
// It replicates https://golang.org/pkg/debug/gosym/#Sym.PackageName, avoiding a
// dependency on debug/gosym.
func packageName(name string) string {
	// A prefix of "type." and "go." is a compiler-generated symbol that doesn't belong to any package.
	if strings.HasPrefix(name, "go.") || strings.HasPrefix(name, "type.") {
		return ""
	}

	pathend := strings.LastIndex(name, "/")
	if pathend < 0 {
		pathend = 0
	}

	if i := strings.Index(name[pathend:], "."); i != -1 {
		return name[:pathend+i]
	}
	return ""
}
