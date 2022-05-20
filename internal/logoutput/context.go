// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package logoutput

import (
	"context"
	"io"
	"os"

	"github.com/rs/zerolog"
	"github.com/spf13/viper"
	"golang.org/x/term"
)

const StampMilliTZ = "Jan _2 15:04:05.000 MST"

func init() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnixMs
}

type logoutputKey string

var _logoutputKey logoutputKey = "fn.log.output"

type OutputTo struct {
	Writer     io.Writer
	WithColors bool
	OutputType OutputType
}

type OutputType string

const OutputText OutputType = "fn.log.output.text"
const OutputJSON OutputType = "fn.log.output.json"

func (o OutputTo) With(w io.Writer) OutputTo {
	return OutputTo{Writer: w, WithColors: o.WithColors, OutputType: o.OutputType}
}

func (o OutputTo) MakeWriter() io.Writer {
	if o.OutputType == OutputJSON {
		return o.Writer
	}

	return zerolog.ConsoleWriter{Out: o.Writer, TimeFormat: StampMilliTZ, NoColor: !o.WithColors}
}

func (o OutputTo) ZeroLogger() *zerolog.Logger {
	l := withZerologWriter(o.MakeWriter())
	return &l
}

func WithOutput(ctx context.Context, o OutputTo) context.Context {
	return withZerolog(context.WithValue(ctx, _logoutputKey, o))
}

func OutputFrom(ctx context.Context) OutputTo {
	if outputTo, ok := ctx.Value(_logoutputKey).(OutputTo); ok {
		return outputTo
	}

	return OutputTo{Writer: os.Stderr, OutputType: OutputText, WithColors: term.IsTerminal(int(os.Stderr.Fd()))}
}

func withZerolog(ctx context.Context) context.Context {
	return OutputFrom(ctx).ZeroLogger().WithContext(ctx)
}

func withZerologWriter(w io.Writer) zerolog.Logger {
	defLevel := zerolog.InfoLevel
	if lvl := viper.GetString("log_level"); lvl != "" {
		l, err := zerolog.ParseLevel(lvl)
		if err == nil {
			defLevel = l
		}
	}

	return zerolog.New(w).With().Timestamp().Logger().Level(defLevel)
}

func WithTee(ctx context.Context, w ...io.Writer) context.Context {
	o := OutputFrom(ctx)
	ctx = WithOutput(ctx, OutputTo{
		WithColors: o.WithColors, // XXX this is wrong
		OutputType: o.OutputType,
		Writer:     io.MultiWriter(append([]io.Writer{o.Writer}, w...)...),
	})
	return withZerolog(ctx) // Reset the zerolog writer.
}
