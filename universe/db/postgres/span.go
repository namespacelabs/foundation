// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package postgres

import (
	"context"
	"errors"
	"io"

	"github.com/jackc/pgx/v4"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"
	statuscodes "google.golang.org/grpc/codes"
	"namespacelabs.dev/foundation/framework/rpcerrors"
)

func withSpan(ctx context.Context, opts commonOpts, name, sql string, f func(context.Context) error) error {
	if opts.t == nil {
		return opts.errw(ctx, f(ctx))
	}

	if s := trace.SpanFromContext(ctx); !s.IsRecording() {
		// (NSL-1740) If this is the first span of the context, then skip it.
		// Traces only containing DB transaction are not useful. We want traces that start with a function call.
		return f(ctx)
	}

	options := []trace.SpanStartOption{trace.WithSpanKind(trace.SpanKindClient)}

	if sql != "" {
		options = append(options, trace.WithAttributes(semconv.DBStatementKey.String(sql)))
	}

	ctx, span := opts.t.Start(ctx, name, options...)
	defer span.End()

	err := checkErr(f(ctx))
	recordErr(span, err)

	return opts.errw(ctx, err)
}

func returnWithSpan[T any](ctx context.Context, opts commonOpts, name, sql string, f func(context.Context) (T, error)) (T, error) {
	if opts.t == nil {
		v, err := f(ctx)
		return v, opts.errw(ctx, err)
	}

	if s := trace.SpanFromContext(ctx); !s.IsRecording() {
		// (NSL-1740) If this is the first span of the context, then skip it.
		// Traces only containing DB transaction are not useful. We want traces that start with a function call.
		return f(ctx)
	}

	options := []trace.SpanStartOption{trace.WithSpanKind(trace.SpanKindClient)}

	if sql != "" {
		options = append(options, trace.WithAttributes(semconv.DBStatementKey.String(sql)))
	}

	ctx, span := opts.t.Start(ctx, name, options...)
	defer span.End()

	value, err := f(ctx)
	err = checkErr(err)
	recordErr(span, err)

	return value, opts.errw(ctx, err)
}

func checkErr(err error) error {
	if errors.Is(err, io.ErrUnexpectedEOF) {
		return rpcerrors.Wrap(statuscodes.Unavailable, err)
	}

	return err
}

func recordErr(span trace.Span, err error) {
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, pgx.ErrNoRows) {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
}
