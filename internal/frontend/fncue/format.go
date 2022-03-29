// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package fncue

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"cuelang.org/go/cue/errors"
	"cuelang.org/go/cue/format"
	"namespacelabs.dev/foundation/internal/fnfs"
)

type Location interface {
	Abs(...string) string
}

type Root interface {
	Abs() string
}

func Format(progress io.Writer, root Root, loc fnfs.Location, name string) {
	p := filepath.Join(root.Abs(), loc.RelPath, name)
	contents, err := ioutil.ReadFile(p)
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Fprintln(progress, "failed to open", p, err)
		}
		return
	}

	formatted, err := format.Source(contents)
	if err != nil {
		switch e := errors.Unwrap(err).(type) {
		case errors.Error:
			format, args := e.Msg()
			fmt.Fprintf(progress, format, args...)
		default:
			fmt.Fprintln(progress, "failed to format", p, err)
		}
	} else if !bytes.Equal(formatted, contents) {
		if err := ioutil.WriteFile(p, formatted, 0644); err != nil {
			fmt.Fprintln(progress, "failed to write", p, err)
		} else {
			fmt.Fprintln(progress, "formatted", p)
		}
	}
}