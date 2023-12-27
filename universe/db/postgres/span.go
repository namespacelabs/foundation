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
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
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

	options := []trace.SpanStartOption{trace.WithSpanKind(trace.SpanKindClient), trace.WithAttributes(opts.TraceAttributes()...)}

	if sql != "" {
		options = append(options, trace.WithAttributes(semconv.DBStatementKey.String(sql)))
	}

	ctx, span := opts.t.Start(ctx, name, options...)
	defer span.End()

	err := checkErr(f(ctx))
	recordErr(span, err)

	return opts.errw(ctx, err)
}

func createSpan(ctx context.Context, opts commonOpts, name, sql string) (context.Context, trace.Span) {
	if opts.t == nil {
		return ctx, nil
	}

	// (NSL-1740) If this is the first span of the context, then skip it.
	// Traces only containing DB transaction are not useful. We want traces that start with a function call.
	if s := trace.SpanFromContext(ctx); !s.IsRecording() {
		return ctx, nil
	}

	options := []trace.SpanStartOption{trace.WithSpanKind(trace.SpanKindClient), trace.WithAttributes(opts.TraceAttributes()...)}

	if sql != "" {
		options = append(options, trace.WithAttributes(semconv.DBStatementKey.String(sql)))
	}

	return opts.t.Start(ctx, name, options...)
}

func returnWithSpan[T any](ctx context.Context, opts commonOpts, name, sql string, f func(context.Context) (T, error)) (T, error) {
	ctx, span := createSpan(ctx, opts, name, sql)
	if span == nil {
		v, err := f(ctx)
		return v, opts.errw(ctx, err)
	}

	defer span.End()

	value, err := f(ctx)
	return value, processError(ctx, span, opts, err)
}

func processError(ctx context.Context, span trace.Span, opts commonOpts, origErr error) error {
	if origErr == nil {
		return nil
	}

	err := checkErr(origErr)
	if span != nil {
		recordErr(span, err)
	}

	return opts.errw(ctx, err)
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
