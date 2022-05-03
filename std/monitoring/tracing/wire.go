// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package tracing

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"sync"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.7.0"
	t "go.opentelemetry.io/otel/trace"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/go/core"
)

var (
	tracingShutdownTimeout = flag.Duration("tracing_shutdown_timeout", 5*time.Second, "How long to wait for the tracer to shutdown.")

	global struct {
		mu             sync.Mutex
		initialized    bool
		exporters      []trace.SpanExporter
		tracerProvider t.TracerProvider // We don't use otel's global, to ensure that dependency order is respected.
	}

	propagators = propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{})
)

type Exporter struct {
	name string
}

func (e Exporter) Register(exp trace.SpanExporter) error {
	global.mu.Lock()
	defer global.mu.Unlock()

	if global.initialized {
		return errors.New("Exporter.Register after initialization was complete")
	}

	global.exporters = append(global.exporters, exp)
	return nil
}

func ProvideExporter(_ context.Context, args *ExporterArgs, _ ExtensionDeps) (Exporter, error) {
	return Exporter{args.Name}, nil
}

func Prepare(ctx context.Context, deps ExtensionDeps) error {
	var opts []trace.TracerProviderOption

	exporters := consumeExporters()
	if len(exporters) == 0 {
		return nil
	}

	for _, exp := range exporters {
		if core.EnvIs(schema.Environment_PRODUCTION) {
			opts = append(opts, trace.WithBatcher(exp, trace.WithBatchTimeout(10*time.Second)))
		} else {
			opts = append(opts, trace.WithSyncer(exp))
		}
	}

	serverResources := core.ServerResourcesFrom(ctx)

	resource, err :=
		resource.New(ctx,
			resource.WithSchemaURL(semconv.SchemaURL),
			resource.WithOS(),
			resource.WithProcessRuntimeName(),
			resource.WithProcessRuntimeVersion(),
			resource.WithProcessRuntimeDescription(),
			resource.WithAttributes(
				semconv.ServiceNameKey.String(deps.ServerInfo.ServerName),
				attribute.String("environment", deps.ServerInfo.EnvName),
				attribute.String("vcs.commit", deps.ServerInfo.GetVcs().GetRevision()),
			),
		)
	if err != nil {
		return err
	}

	opts = append(opts, trace.WithResource(resource))

	provider := trace.NewTracerProvider(opts...)
	serverResources.Add(close{provider})

	deps.Interceptors.ForServer(
		otelgrpc.UnaryServerInterceptor(otelgrpc.WithTracerProvider(provider), otelgrpc.WithPropagators(propagators)),
		otelgrpc.StreamServerInterceptor(otelgrpc.WithTracerProvider(provider), otelgrpc.WithPropagators(propagators)))

	deps.Interceptors.ForClient(
		otelgrpc.UnaryClientInterceptor(otelgrpc.WithTracerProvider(provider), otelgrpc.WithPropagators(propagators)),
		otelgrpc.StreamClientInterceptor(otelgrpc.WithTracerProvider(provider), otelgrpc.WithPropagators(propagators)),
	)

	deps.Middleware.Add(func(h http.Handler) http.Handler {
		return otelhttp.NewHandler(h, "",
			otelhttp.WithTracerProvider(provider),
			otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
				return fmt.Sprintf("%s %s", r.Method, r.URL.Path)
			}))
	})

	global.mu.Lock()
	global.tracerProvider = provider
	global.mu.Unlock()

	return nil
}

func consumeExporters() []trace.SpanExporter {
	global.mu.Lock()
	defer global.mu.Unlock()

	exporters := global.exporters
	global.initialized = true
	return exporters
}

type close struct {
	tp *trace.TracerProvider
}

func (c close) Close(ctx context.Context) error {
	ctxWithTimeout, cancel := context.WithTimeout(ctx, *tracingShutdownTimeout)
	defer cancel()
	return c.tp.Shutdown(ctxWithTimeout)
}

func getTracerProvider() (t.TracerProvider, error) {
	global.mu.Lock()
	defer global.mu.Unlock()

	if !global.initialized {
		return nil, errors.New("tried to get a non-initialized TracerProvider; you need to use initializeAfter")
	}

	return global.tracerProvider, nil
}

type DeferredTracerProvider struct{}

func (DeferredTracerProvider) GetTracerProvider() (t.TracerProvider, error) {
	return getTracerProvider()
}

func ProvideTracerProvider(context.Context, *NoArgs, ExtensionDeps) (DeferredTracerProvider, error) {
	return DeferredTracerProvider{}, nil
}
