// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package fncue

import (
	"context"
	"io"
	"io/fs"
	"os"

	"cuelang.org/go/cue/errors"
	"cuelang.org/go/cue/format"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
)

type Location interface {
	Abs(...string) string
}

type Root interface {
	Abs() string
}

func Format(ctx context.Context, fsfs fnfs.ReadWriteFS, loc fnfs.Location, name string, opts fnfs.WriteFileExtendedOpts) error {
	contents, err := fs.ReadFile(fsfs, loc.Rel(name))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	opts.CompareContents = true
	opts.EnsureFileMode = false

	return fnfs.WriteFileExtended(ctx, fsfs, loc.Rel(name), 0644, opts, func(w io.Writer) error {
		return FormatSource(loc, w, contents)
	})
}

func FormatSource(loc fnerrors.Location, w io.Writer, contents []byte) error {
	formatted, err := format.Source(contents)
	if err != nil {
		switch e := errors.Unwrap(err).(type) {
		case errors.Error:
			format, args := e.Msg()
			return fnerrors.Wrapf(loc, err, format, args...)
		default:
			return fnerrors.Wrapf(loc, err, "failed to format")
		}
	}

	_, err = w.Write(formatted)
	return err
}
