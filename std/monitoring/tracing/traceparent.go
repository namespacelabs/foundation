// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package tracing

import (
	"context"

	"go.opentelemetry.io/otel/propagation"
)

const privateTraceparent = "x-namespace-trace-parent"

type NamespaceTraceParent struct{}

var _ propagation.TextMapPropagator = NamespaceTraceParent{}

func (NamespaceTraceParent) Inject(ctx context.Context, carrier propagation.TextMapCarrier) {
	// Do nothing.
}

func (NamespaceTraceParent) Extract(ctx context.Context, carrier propagation.TextMapCarrier) context.Context {
	p := carrier.Get(privateTraceparent)
	if p != "" {
		return propagation.TraceContext{}.Extract(ctx, propagation.MapCarrier{"traceparent": p})
	}

	return ctx
}

func (NamespaceTraceParent) Fields() []string {
	return []string{privateTraceparent}
}
