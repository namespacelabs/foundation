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
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
	statuscodes "google.golang.org/grpc/codes"
	"namespacelabs.dev/foundation/framework/rpcerrors"
)

func withSpan(ctx context.Context, tracer trace.Tracer, name, sql string, f func(context.Context) error) error {
	if tracer == nil {
		return f(ctx)
	}

	options := []trace.SpanStartOption{trace.WithSpanKind(trace.SpanKindClient)}

	if sql != "" {
		options = append(options, trace.WithAttributes(semconv.DBStatementKey.String(sql)))
	}

	ctx, span := tracer.Start(ctx, name, options...)
	defer span.End()

	err := checkErr(f(ctx))
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, pgx.ErrNoRows) {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}

	return err
}

func returnWithSpan[T any](ctx context.Context, tracer trace.Tracer, name, sql string, f func(context.Context) (T, error)) (T, error) {
	if tracer == nil {
		return f(ctx)
	}

	options := []trace.SpanStartOption{trace.WithSpanKind(trace.SpanKindClient)}

	if sql != "" {
		options = append(options, trace.WithAttributes(semconv.DBStatementKey.String(sql)))
	}

	ctx, span := tracer.Start(ctx, name, options...)
	defer span.End()

	value, err := f(ctx)
	err = checkErr(err)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, pgx.ErrNoRows) {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}

	return value, err
}

func checkErr(err error) error {
	if errors.Is(err, io.ErrUnexpectedEOF) {
		return rpcerrors.Wrap(statuscodes.Unavailable, err)
	}

	return err
}
