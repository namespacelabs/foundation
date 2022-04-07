// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package tracing

import (
	"context"
	"flag"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/sdk/trace"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/go/core"
)

// XXX we're coupling two things in this node that should be separate: the
// grpc<->tracing binding, and the initialization of a tracer. The latter
// should be really in a "jeager" node, which provides a tracer that would
// then be consumed by this node.

var (
	jaegerEndpoint        = flag.String("jaeger_collector_endpoint", "", "Where to push jaeger data to.")
	jaegerShutdownTimeout = flag.Duration("jaeger_shutdown_timeout", 5*time.Second, "How long to wait for the tracer to shutdown.")
)

func Prepare(ctx context.Context, deps SingletonDeps) error {
	if *jaegerEndpoint == "" {
		return nil
	}

	exp, err := jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(*jaegerEndpoint)))
	if err != nil {
		return err
	}

	var opts []trace.TracerProviderOption

	// Record information about this application in an Resource.
	// opts = append(opts,
	// 	tracesdk.WithResource(resource.NewWithAttributes(
	// 		attribute.String("environment", environment),
	// 		attribute.Int64("ID", id),
	// 	)))

	if core.EnvIs(schema.Environment_PRODUCTION) {
		opts = append(opts, trace.WithBatcher(exp, trace.WithBatchTimeout(10*time.Second)))
	} else {
		opts = append(opts, trace.WithSyncer(exp))
	}

	tp := trace.NewTracerProvider(opts...)

	core.ServerResourcesFrom(ctx).Add(close{tp})

	deps.Interceptors.Add(otelgrpc.UnaryServerInterceptor(otelgrpc.WithTracerProvider(tp)),
		otelgrpc.StreamServerInterceptor(otelgrpc.WithTracerProvider(tp)))

	return nil
}

type close struct {
	tp *trace.TracerProvider
}

func (c close) Close(ctx context.Context) error {
	ctxWithTimeout, cancel := context.WithTimeout(ctx, *jaegerShutdownTimeout)
	defer cancel()
	return c.tp.Shutdown(ctxWithTimeout)
}
