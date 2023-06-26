// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package tracing

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
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
	ctx, span := tracer.Start(ctx, name.name, trace.WithAttributes(name.attributes...))
	defer span.End(trace.WithStackTrace(true))

	err := callback(ctx)
	if err != nil {
		span.RecordError(err)
	}

	return err
}

func Collect1[T any](ctx context.Context, tracer trace.Tracer, name Collected, callback func(context.Context) (T, error)) (T, error) {
	ctx, span := tracer.Start(ctx, name.name, trace.WithAttributes(name.attributes...))
	defer span.End(trace.WithStackTrace(true))

	value, err := callback(ctx)
	if err != nil {
		span.RecordError(err)
	}

	return value, err
}
