// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package tracing

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	oteltrace "go.opentelemetry.io/otel/trace"
	t "go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
	"golang.org/x/exp/slices"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/core/types"
	"namespacelabs.dev/foundation/std/go/core"
)

var (
	tracingShutdownTimeout = flag.Duration("tracing_shutdown_timeout", 5*time.Second, "How long to wait for the tracer to shutdown.")

	global struct {
		mu              sync.Mutex
		initialized     bool
		exporters       []trace.SpanExporter
		metricExporters []metric.Exporter
		detectors       []resource.Detector
		tracerProvider  t.TracerProvider // We don't use otel's global, to ensure that dependency order is respected.
	}

	propagators = propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{})

	instanceID = uuid.New().String()
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

func (e Exporter) RegisterMetrics(exp metric.Exporter) error {
	global.mu.Lock()
	defer global.mu.Unlock()

	if global.initialized {
		return errors.New("Exporter.Register after initialization was complete")
	}

	global.metricExporters = append(global.metricExporters, exp)
	return nil
}

func ProvideExporter(_ context.Context, args *ExporterArgs, _ ExtensionDeps) (Exporter, error) {
	return Exporter{args.Name}, nil
}

type Detector struct {
	name string
}

func (e Detector) Register(detector resource.Detector) error {
	global.mu.Lock()
	defer global.mu.Unlock()

	if global.initialized {
		return errors.New("Detector.Register after initialization was complete")
	}

	global.detectors = append(global.detectors, detector)
	return nil
}

func ProvideDetector(_ context.Context, args *DetectorArgs, _ ExtensionDeps) (Detector, error) {
	return Detector{args.Name}, nil
}

func CreateResource(ctx context.Context, serverInfo *types.ServerInfo, detectors []resource.Detector) (*resource.Resource, error) {
	return resource.New(ctx,
		resource.WithSchemaURL(semconv.SchemaURL),
		resource.WithOS(),
		resource.WithProcessRuntimeName(),
		resource.WithProcessRuntimeVersion(),
		resource.WithProcessRuntimeDescription(),
		resource.WithDetectors(detectors...),
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serverInfo.ServerName),
			semconv.ServiceVersionKey.String(serverInfo.GetVcs().GetRevision()),
			semconv.ServiceInstanceIDKey.String(instanceID),
			semconv.DeploymentEnvironmentKey.String(serverInfo.EnvName),
			attribute.String("environment", serverInfo.EnvName),
		),
	)
}

func CreateProvider(ctx context.Context, serverInfo *types.ServerInfo, exporters []trace.SpanExporter, detectors []resource.Detector) (t.TracerProvider, core.CtxCloseable, error) {
	if len(exporters) == 0 {
		if os.Getenv("FOUNDATION_TRACE_TO_STDOUT") != "1" {
			return noop.NewTracerProvider(), nil, nil
		}

		out, err := stdouttrace.New()
		if err != nil {
			return nil, nil, err
		}

		exporters = append(exporters, out)
	}

	var opts []trace.TracerProviderOption

	for _, exp := range exporters {
		if core.EnvIs(schema.Environment_PRODUCTION) {
			opts = append(opts, trace.WithBatcher(exp, trace.WithBatchTimeout(10*time.Second)))
		} else {
			opts = append(opts, trace.WithSyncer(exp))
		}
	}

	resource, err := CreateResource(ctx, serverInfo, detectors)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create resource: %w", err)
	}

	opts = append(opts, trace.WithResource(resource))

	tp := trace.NewTracerProvider(opts...)

	return tp, close{tp}, nil
}

func Prepare(ctx context.Context, deps ExtensionDeps) error {
	provider, closeable, err := CreateProvider(ctx, deps.ServerInfo, consumeExporters(), consumeDetectors())
	if err != nil {
		return err
	}

	serverResources := core.ServerResourcesFrom(ctx)

	if serverResources != nil && closeable != nil {
		serverResources.Add(closeable)
	}

	filter := func(*otelgrpc.InterceptorInfo) bool { return true } // By default we trace every gRPC method
	if skipStr := os.Getenv("FOUNDATION_GRPCTRACE_SKIP_METHODS"); skipStr != "" {
		skipTraces := strings.Split(skipStr, ",")
		filter = func(info *otelgrpc.InterceptorInfo) bool {
			if info != nil {
				if slices.Contains(skipTraces, info.Method) {
					return false
				}
				if info.UnaryServerInfo != nil && slices.Contains(skipTraces, info.UnaryServerInfo.FullMethod) {
					return false
				}
				if info.StreamServerInfo != nil && slices.Contains(skipTraces, info.StreamServerInfo.FullMethod) {
					return false
				}
			}
			return true
		}
	}

	deps.Interceptors.ForServer(
		otelgrpc.UnaryServerInterceptor(otelgrpc.WithTracerProvider(provider), otelgrpc.WithPropagators(propagators),
			otelgrpc.WithMessageEvents(), otelgrpc.WithInterceptorFilter(filter)),
		otelgrpc.StreamServerInterceptor(otelgrpc.WithTracerProvider(provider), otelgrpc.WithPropagators(propagators),
			otelgrpc.WithMessageEvents(), otelgrpc.WithInterceptorFilter(filter)))

	deps.Interceptors.ForClient(
		otelgrpc.UnaryClientInterceptor(otelgrpc.WithTracerProvider(provider), otelgrpc.WithPropagators(propagators),
			otelgrpc.WithMessageEvents(), otelgrpc.WithInterceptorFilter(filter)),
		otelgrpc.StreamClientInterceptor(otelgrpc.WithTracerProvider(provider), otelgrpc.WithPropagators(propagators),
			otelgrpc.WithMessageEvents(), otelgrpc.WithInterceptorFilter(filter)),
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

func consumeMetricsExporters() []metric.Exporter {
	global.mu.Lock()
	defer global.mu.Unlock()

	exporters := global.metricExporters
	global.initialized = true
	return exporters
}

func consumeDetectors() []resource.Detector {
	global.mu.Lock()
	defer global.mu.Unlock()

	detectors := global.detectors
	global.initialized = true
	return detectors
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

type DeferredTracerProvider interface {
	GetTracerProvider() (t.TracerProvider, error)
}

func ProvideTracerProvider(context.Context, *NoArgs, ExtensionDeps) (DeferredTracerProvider, error) {
	return deferredTracerProvider{}, nil
}

type deferredTracerProvider struct{}

func (deferredTracerProvider) GetTracerProvider() (t.TracerProvider, error) {
	return getTracerProvider()
}

func Tracer(pkg *core.Package, p DeferredTracerProvider) (oteltrace.Tracer, error) {
	tracer, err := p.GetTracerProvider()
	if err != nil {
		return nil, err
	}

	if tracer == nil {
		return nil, nil
	}

	return tracer.Tracer(pkg.PackageName), nil
}

func MustTracer(pkg *core.Package, p DeferredTracerProvider) oteltrace.Tracer {
	t, err := Tracer(pkg, p)
	if err != nil {
		panic(err)
	}

	return t
}
