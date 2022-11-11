// Copyright (c) 2015, Dave Cheney <dave@cheney.net>
// All rights reserved.

// Adapted from https://github.com/pkg/errors.

package stacktrace

import (
	"fmt"
	"io"
	"path"
	"runtime"
	"strconv"
	"strings"
)

// Frame represents a program counter inside a stack frame.
// For historical reasons if Frame is interpreted as a uintptr
// its value represents the program counter + 1.
type Frame uintptr

// pc returns the program counter for this frame;
// multiple frames may have the same PC value.
func (f Frame) pc() uintptr { return uintptr(f) - 1 }

// File returns the full path to the file that contains the
// function for this Frame's pc.
func (f Frame) File() string {
	fn := runtime.FuncForPC(f.pc())
	if fn == nil {
		return "unknown"
	}
	file, _ := fn.FileLine(f.pc())
	return file
}

// Line returns the line number of source code of the
// function for this Frame's pc.
func (f Frame) Line() int {
	fn := runtime.FuncForPC(f.pc())
	if fn == nil {
		return 0
	}
	_, line := fn.FileLine(f.pc())
	return line
}

// Name returns the name of this function, if known.
func (f Frame) Name() string {
	fn := runtime.FuncForPC(f.pc())
	if fn == nil {
		return "unknown"
	}
	return fn.Name()
}

// Format formats the frame according to the fmt.Formatter interface.
//
//	%s    source file
//	%d    source line
//	%n    function name
//	%v    equivalent to %s:%d
//
// Format accepts flags that alter the printing of some verbs, as follows:
//
//	%+s   function name and path of source file relative to the compile time
//	      GOPATH separated by \n\t (<funcname>\n\t<path>)
//	%+v   equivalent to %+s:%d
func (f Frame) Format(s fmt.State, verb rune) {
	switch verb {
	case 's':
		switch {
		case s.Flag('+'):
			writeString(s, f.Name())
			writeString(s, "\n\t")
			writeString(s, f.File())
		default:
			writeString(s, path.Base(f.File()))
		}
	case 'd':
		writeString(s, strconv.Itoa(f.Line()))
	case 'n':
		writeString(s, funcname(f.Name()))
	case 'v':
		f.Format(s, 's')
		writeString(s, ":")
		f.Format(s, 'd')
	}
}

// errcheck safe wrapper for io.WriteString.
func writeString(w io.Writer, s string) {
	_, _ = io.WriteString(w, s)
}

// MarshalText formats a stacktrace Frame as a text string. The output is the
// same as that of fmt.Sprintf("%+v", f), but without newlines or tabs.
func (f Frame) MarshalText() ([]byte, error) {
	name := f.Name()
	if name == "unknown" {
		return []byte(name), nil
	}
	return []byte(fmt.Sprintf("%s %s:%d", name, f.File(), f.Line())), nil
}

// StackTrace is stack of Frames from innermost (newest) to outermost (oldest).
type StackTrace []Frame

// New returns a new StackTrace after filtering out uninteresting frames
// from runtime.Callers.
func New() StackTrace {
	return NewWithSkip(1)
}

// New returns a new StackTrace after filtering out uninteresting frames
// from runtime.Callers.
func NewWithSkip(k int) StackTrace {
	const depth = 32
	var pcs [depth]uintptr

	// We skip 3 frames: 0 identifies the frame for Callers,
	// 1 identifies the call below, and 2 identifies the `New`
	// invocation above all of which are uninteresting for the
	// user.
	n := runtime.Callers(k+3, pcs[:])
	pcslice := pcs[0:n]
	frames := make([]Frame, len(pcslice))
	for i := 0; i < len(frames); i++ {
		frames[i] = Frame((pcslice)[i])
	}
	return frames
}

// Format formats the stack of Frames according to the fmt.Formatter interface.
//
//	%s	lists source files for each Frame in the stack
//	%v	lists the source file and line number for each Frame in the stack
//
// Format accepts flags that alter the printing of some verbs, as follows:
//
//	%+v   Prints filename, function, and line number for each Frame in the stack.
func (st StackTrace) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		switch {
		case s.Flag('+'):
			for _, f := range st {
				writeString(s, "\n")
				f.Format(s, verb)
			}
		case s.Flag('#'):
			fmt.Fprintf(s, "%#v", []Frame(st))
		default:
			st.formatSlice(s, verb)
		}
	case 's':
		st.formatSlice(s, verb)
	}
}

// formatSlice will format this StackTrace into the given buffer as a slice of
// Frame, only valid when called with '%s' or '%v'.
func (st StackTrace) formatSlice(s fmt.State, verb rune) {
	writeString(s, "[")
	for i, f := range st {
		if i > 0 {
			writeString(s, " ")
		}
		f.Format(s, verb)
	}
	writeString(s, "]")
}

// funcname removes the path prefix component of a function's name reported by func.Name().
func funcname(name string) string {
	i := strings.LastIndex(name, "/")
	name = name[i+1:]
	i = strings.Index(name, ".")
	return name[i+1:]
}
