// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package tracing

import (
	"context"
	"time"

	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	grpc_codes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Collected struct {
	name       string
	attributes []attribute.KeyValue
	newroot    bool
}

func Name(name string) Collected {
	return Collected{name: name}
}

func (c Collected) Attribute(kv ...attribute.KeyValue) Collected {
	copy := c
	copy.attributes = append(c.attributes, kv...)
	return copy
}

func (c Collected) NewRoot() Collected {
	copy := c
	copy.newroot = true
	return copy
}

func Collect0(ctx context.Context, tracer trace.Tracer, name Collected, callback func(context.Context) error, opts ...trace.SpanStartOption) error {
	if tracer == nil {
		return callback(ctx)
	}

	opts = append([]trace.SpanStartOption{trace.WithAttributes(name.attributes...)}, opts...)
	if name.newroot {
		opts = append(opts, trace.WithNewRoot())
	}

	ctx, span := tracer.Start(ctx, name.name, opts...)
	defer span.End(trace.WithStackTrace(true))

	err := callback(ctx)
	if err != nil {
		maybeCollectError(span, err)
	}

	return err
}

func Collect1[T any](ctx context.Context, tracer trace.Tracer, name Collected, callback func(context.Context) (T, error), opts ...trace.SpanStartOption) (T, error) {
	if tracer == nil {
		return callback(ctx)
	}

	if !trace.SpanFromContext(ctx).IsRecording() && !name.newroot {
		return callback(ctx)
	}

	opts = append([]trace.SpanStartOption{trace.WithAttributes(name.attributes...)}, opts...)
	if name.newroot {
		opts = append(opts, trace.WithNewRoot())
	}

	ctx, span := tracer.Start(ctx, name.name, opts...)
	defer span.End(trace.WithStackTrace(true))

	value, err := callback(ctx)
	if err != nil {
		maybeCollectError(span, err)
	}

	return value, err
}

func Collect2[T any, R any](ctx context.Context, tracer trace.Tracer, name Collected, callback func(context.Context) (T, R, error), opts ...trace.SpanStartOption) (T, R, error) {
	if tracer == nil {
		return callback(ctx)
	}

	opts = append([]trace.SpanStartOption{trace.WithAttributes(name.attributes...)}, opts...)
	if name.newroot {
		opts = append(opts, trace.WithNewRoot())
	}

	ctx, span := tracer.Start(ctx, name.name, opts...)
	defer span.End(trace.WithStackTrace(true))

	value0, value1, err := callback(ctx)
	if err != nil {
		maybeCollectError(span, err)
	}

	return value0, value1, err
}

func CollectAndLog0(ctx context.Context, tracer trace.Tracer, name Collected, callback func(context.Context) error, opts ...trace.SpanStartOption) error {
	zlb := loggerFromAttrs(zerolog.Ctx(ctx).With(), name.attributes).Logger()

	zlb.Info().Msgf("%s", name.name)
	err := Collect0(zlb.WithContext(ctx), tracer, name, callback, opts...)
	if err != nil {
		zlb.Err(err).Msgf("%s (failed)", name.name)
	}
	return err
}

func CollectAndLog1[T any](ctx context.Context, tracer trace.Tracer, name Collected, callback func(context.Context) (T, error), opts ...trace.SpanStartOption) (T, error) {
	zlb := loggerFromAttrs(zerolog.Ctx(ctx).With(), name.attributes).Logger()

	zlb.Info().Msgf("%s", name.name)
	v, err := Collect1[T](zlb.WithContext(ctx), tracer, name, callback, opts...)
	if err != nil {
		zlb.Err(err).Msgf("%s (failed)", name.name)
	}
	return v, err
}

func CollectAndLogDuration0(ctx context.Context, tracer trace.Tracer, name Collected, callback func(context.Context) error, opts ...trace.SpanStartOption) error {
	zlb := loggerFromAttrs(zerolog.Ctx(ctx).With(), name.attributes).Logger()

	t := time.Now()
	zlb.Info().Msgf("%s (started)", name.name)
	err := Collect0(zlb.WithContext(ctx), tracer, name, callback, opts...)
	if err != nil {
		zlb.Err(err).Msgf("%s (failed)", name.name)
	} else {
		zlb.Info().Dur("took", time.Since(t)).Msgf("%s (done)", name.name)
	}
	return err
}

func CollectAndLogDuration1[T any](ctx context.Context, tracer trace.Tracer, name Collected, callback func(context.Context) (T, error), opts ...trace.SpanStartOption) (T, error) {
	zlb := loggerFromAttrs(zerolog.Ctx(ctx).With(), name.attributes).Logger()

	t := time.Now()
	zlb.Info().Msgf("%s (started)", name.name)
	v, err := Collect1[T](zlb.WithContext(ctx), tracer, name, callback, opts...)
	if err != nil {
		zlb.Err(err).Msgf("%s (failed)", name.name)
	} else {
		zlb.Info().Dur("took", time.Since(t)).Msgf("%s (done)", name.name)
	}
	return v, err
}

func loggerFromAttrs(zlb zerolog.Context, attrs []attribute.KeyValue) zerolog.Context {
	for _, attr := range attrs {
		zlb = zlb.Interface(string(attr.Key), attr.Value.AsInterface())
	}

	return zlb
}

func maybeCollectError(span trace.Span, err error) {
	span.RecordError(err)
	s, ok := status.FromError(err)
	if ok && s.Code() != grpc_codes.OK {
		span.SetStatus(codes.Error, s.Message())
	}
}
