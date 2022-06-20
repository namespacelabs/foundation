// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package logoutput

import (
	"context"
	"io"
)

const StampMilliTZ = "Jan _2 15:04:05.000 MST"

type logoutputKey string

var _logoutputKey logoutputKey = "fn.log.output"

type OutputTo struct {
	Writer     io.Writer
	WithColors bool
}

func (o OutputTo) With(w io.Writer) OutputTo {
	return OutputTo{Writer: w, WithColors: o.WithColors}
}

// XXX remove this as there are no leftover consumers.
func WithOutput(ctx context.Context, o OutputTo) context.Context {
	return context.WithValue(ctx, _logoutputKey, o)
}
