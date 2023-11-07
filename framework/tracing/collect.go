// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package tracing

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	grpc_codes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Collected struct {
	name       string
	attributes []attribute.KeyValue
}

func Name(name string) Collected {
	return Collected{name: name}
}

func (c Collected) Attribute(kv ...attribute.KeyValue) Collected {
	return Collected{name: c.name, attributes: append(c.attributes, kv...)}
}

func Collect0(ctx context.Context, tracer trace.Tracer, name Collected, callback func(context.Context) error) error {
	if tracer == nil {
		return callback(ctx)
	}

	ctx, span := tracer.Start(ctx, name.name, trace.WithAttributes(name.attributes...))
	defer span.End(trace.WithStackTrace(true))

	err := callback(ctx)
	if err != nil {
		maybeCollectError(span, err)
	}

	return err
}

func Collect1[T any](ctx context.Context, tracer trace.Tracer, name Collected, callback func(context.Context) (T, error)) (T, error) {
	if tracer == nil {
		return callback(ctx)
	}

	ctx, span := tracer.Start(ctx, name.name, trace.WithAttributes(name.attributes...))
	defer span.End(trace.WithStackTrace(true))

	value, err := callback(ctx)
	if err != nil {
		maybeCollectError(span, err)
	}

	return value, err
}

func maybeCollectError(span trace.Span, err error) {
	span.RecordError(err)
	s, _ := status.FromError(err)
	statusCode, msg := serverStatus(s)
	span.SetStatus(statusCode, msg)
}

func serverStatus(grpcStatus *status.Status) (codes.Code, string) {
	switch grpcStatus.Code() {
	case grpc_codes.Unknown,
		grpc_codes.DeadlineExceeded,
		grpc_codes.Unimplemented,
		grpc_codes.Internal,
		grpc_codes.Unavailable,
		grpc_codes.DataLoss,
		grpc_codes.ResourceExhausted,
		grpc_codes.FailedPrecondition:
		return codes.Error, grpcStatus.Message()
	default:
		return codes.Unset, ""
	}
}
