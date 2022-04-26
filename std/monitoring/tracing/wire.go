// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package tracing

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.7.0"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/go/core"
	"namespacelabs.dev/foundation/std/go/grpc/interceptors"
	"namespacelabs.dev/foundation/std/go/http/middleware"
)

var (
	tracingShutdownTimeout = flag.Duration("tracing_shutdown_timeout", 5*time.Second, "How long to wait for the tracer to shutdown.")
)

type TraceProvider struct {
	name            string
	resource        *resource.Resource
	serverResources *core.ServerResources
	interceptors    interceptors.Registration
	middleware      middleware.Middleware
}

func (tp TraceProvider) Setup(exp trace.SpanExporter) error {
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

	opts = append(opts, trace.WithResource(tp.resource))

	provider := trace.NewTracerProvider(opts...)
	tp.serverResources.Add(close{provider})

	tp.interceptors.Add(otelgrpc.UnaryServerInterceptor(otelgrpc.WithTracerProvider(provider)),
		otelgrpc.StreamServerInterceptor(otelgrpc.WithTracerProvider(provider)))

	tp.middleware.Add(func(h http.Handler) http.Handler {
		return otelhttp.NewHandler(h, "",
			otelhttp.WithTracerProvider(provider),
			otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
				return fmt.Sprintf("%s %s", r.Method, r.URL.Path)
			}))
	})

	return nil
}

func ProvideTraceProvider(ctx context.Context, args *TraceProviderArgs, deps ExtensionDeps) (TraceProvider, error) {
	serverResources := core.ServerResourcesFrom(ctx)

	resource, err :=
		resource.New(ctx,
			resource.WithSchemaURL(semconv.SchemaURL),
			resource.WithOS(),
			resource.WithProcess(),
			resource.WithAttributes(
				semconv.ServiceNameKey.String(deps.ServerInfo.ServerName),
				attribute.String("environment", deps.ServerInfo.EnvName),
				attribute.String("vcs.commit", deps.ServerInfo.GetVcs().GetRevision()),
			),
		)

	if err != nil {
		return TraceProvider{}, err
	}

	return TraceProvider{args.Name, resource, serverResources, deps.Interceptors, deps.Middleware}, nil
}

type close struct {
	tp *trace.TracerProvider
}

func (c close) Close(ctx context.Context) error {
	ctxWithTimeout, cancel := context.WithTimeout(ctx, *tracingShutdownTimeout)
	defer cancel()
	return c.tp.Shutdown(ctxWithTimeout)
}
