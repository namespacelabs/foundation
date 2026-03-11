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
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	oteltrace "go.opentelemetry.io/otel/trace"
	t "go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
	"golang.org/x/exp/slices"
	"google.golang.org/grpc"
	"google.golang.org/grpc/stats"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/core/types"
	"namespacelabs.dev/foundation/std/go/core"
	"namespacelabs.dev/foundation/std/monitoring/tracing/filter"
)

var (
	tracingShutdownTimeout = flag.Duration("tracing_shutdown_timeout", 5*time.Second, "How long to wait for the tracer to shutdown.")

	global struct {
		mu              sync.Mutex
		initialized     bool
		exporters       []sdktrace.SpanExporter
		metricExporters []metric.Exporter
		detectors       []resource.Detector
		tracerProvider  t.TracerProvider // We don't use otel's global, to ensure that dependency order is respected.
	}

	instanceID = uuid.New().String()
)

type Exporter struct {
	name string
}

func (e Exporter) Register(exp sdktrace.SpanExporter) error {
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
	serviceName := serverInfo.ServerName
	if serverInfo.GetTelemetryResource().GetServiceName() != "" {
		serviceName = serverInfo.GetTelemetryResource().GetServiceName()
	}

	return resource.New(ctx,
		resource.WithSchemaURL(semconv.SchemaURL),
		resource.WithOS(),
		resource.WithProcessRuntimeName(),
		resource.WithProcessRuntimeVersion(),
		resource.WithProcessRuntimeDescription(),
		resource.WithDetectors(detectors...),
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
			semconv.ServiceVersionKey.String(serverInfo.GetVcs().GetRevision()),
			semconv.ServiceInstanceIDKey.String(instanceID),
			semconv.DeploymentEnvironmentName(serverInfo.EnvName),
			attribute.String("environment", serverInfo.EnvName),
		),
	)
}

func CreateProvider(ctx context.Context, serverInfo *types.ServerInfo, exporters []sdktrace.SpanExporter, detectors []resource.Detector) (t.TracerProvider, core.CtxCloseable, error) {
	if os.Getenv("FOUNDATION_TRACE_TO_STDOUT") == "1" {
		out, err := stdouttrace.New()
		if err != nil {
			return nil, nil, err
		}

		exporters = append(exporters, out)
	}

	if traceToFile := strings.TrimSpace(os.Getenv("FOUNDATION_TRACE_TO_FILE")); traceToFile != "" {
		f, err := os.OpenFile(traceToFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to open FOUNDATION_TRACE_TO_FILE target: %w", err)
		}

		out, err := stdouttrace.New(stdouttrace.WithWriter(f))
		if err != nil {
			_ = f.Close()
			return nil, nil, err
		}

		exporters = append(exporters, fileExporter{SpanExporter: out, file: f})
	}

	if len(exporters) == 0 {
		return noop.NewTracerProvider(), nil, nil
	}

	var opts []sdktrace.TracerProviderOption

	for _, exp := range exporters {
		if core.EnvIs(schema.Environment_PRODUCTION) {
			opts = append(opts, sdktrace.WithBatcher(exp, sdktrace.WithBatchTimeout(10*time.Second)))
		} else {
			opts = append(opts, sdktrace.WithSyncer(exp))
		}
	}

	resource, err := CreateResource(ctx, serverInfo, detectors)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create resource: %w", err)
	}

	opts = append(opts, sdktrace.WithResource(resource))

	if sampler := filter.FilterFromEnv(filter.DefaultEnvKey); sampler != nil {
		opts = append(opts, sdktrace.WithSampler(sampler))
	}

	tp := sdktrace.NewTracerProvider(opts...)

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

	grpcFilter := func(*stats.RPCTagInfo) bool { return true } // By default we trace every gRPC method
	if skipStr := os.Getenv("FOUNDATION_GRPCTRACE_SKIP_METHODS"); skipStr != "" {
		skipTraces := strings.Split(skipStr, ",")
		grpcFilter = func(info *stats.RPCTagInfo) bool {
			if info != nil && slices.Contains(skipTraces, info.FullMethodName) {
				return false
			}

			return true
		}
	}

	srvh := otelgrpc.NewServerHandler(
		otelgrpc.WithTracerProvider(provider),
		otelgrpc.WithPropagators(Propagators()),
		otelgrpc.WithFilter(grpcFilter),
	)

	deps.Interceptors.ServerBuilder().
		WithHandler(srvh).
		WithInterceptors(func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
			if deadline, ok := ctx.Deadline(); ok {
				t.SpanFromContext(ctx).SetAttributes(attribute.Int64("rpc.deadline_left_ms", time.Until(deadline).Milliseconds()))
			}

			return handler(ctx, req)
		}, func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
			ctx := ss.Context()
			if deadline, ok := ctx.Deadline(); ok {
				t.SpanFromContext(ctx).SetAttributes(attribute.Int64("rpc.deadline_left_ms", time.Until(deadline).Milliseconds()))
			}

			return handler(srv, ss)
		}).
		Register()

	clih := otelgrpc.NewClientHandler(
		otelgrpc.WithTracerProvider(provider),
		otelgrpc.WithPropagators(Propagators()),
		otelgrpc.WithFilter(grpcFilter),
	)

	deps.Interceptors.HandlerForClient(clih)

	httpFilter := func(r *http.Request) bool { return true }
	// FOUNDATION_HTTP_TRACE_SKIP_CONTENT_TYPES is a comma-separated list of Content-Type prefixes
	// to exclude from HTTP-level tracing. gRPC and Connect requests are already traced by their own
	// interceptors (otelgrpc, otelconnect), so the otelhttp spans are redundant for those protocols.
	// Recommended value: "application/grpc,application/proto,application/connect+"
	if skipStr := os.Getenv("FOUNDATION_HTTP_TRACE_SKIP_CONTENT_TYPES"); skipStr != "" {
		skipPrefixes := strings.Split(skipStr, ",")
		base := httpFilter
		httpFilter = func(r *http.Request) bool {
			ct := r.Header.Get("Content-Type")
			for _, prefix := range skipPrefixes {
				if strings.HasPrefix(ct, prefix) {
					return false
				}
			}
			return base(r)
		}
	}

	// FOUNDATION_HTTP_TRACE_SKIP_HEADERS is a comma-separated list of header names. If any of these
	// headers are present in the request, the HTTP-level span is skipped.
	// This is useful for protocols that use standard Content-Types (e.g. application/json) but
	// include a distinguishing header. For example, Connect unary RPCs with JSON encoding
	// send "Connect-Protocol-Version: 1".
	// Recommended value: "Connect-Protocol-Version"
	if skipStr := os.Getenv("FOUNDATION_HTTP_TRACE_SKIP_HEADERS"); skipStr != "" {
		skipHeaders := strings.Split(skipStr, ",")
		base := httpFilter
		httpFilter = func(r *http.Request) bool {
			for _, h := range skipHeaders {
				if r.Header.Get(h) != "" {
					return false
				}
			}
			return base(r)
		}
	}

	if skipStr := os.Getenv("FOUNDATION_HTTP_TRACE_SKIP_PATHS"); skipStr != "" {
		skipPaths := strings.Split(skipStr, ",")
		base := httpFilter
		httpFilter = func(r *http.Request) bool {
			if slices.Contains(skipPaths, r.URL.Path) {
				return false
			}
			return base(r)
		}
	}

	deps.Middleware.Add(func(h http.Handler) http.Handler {
		return otelhttp.NewHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// add referer
			span := t.SpanFromContext(r.Context())
			if span.IsRecording() {
				if referer := r.Header.Get("Referer"); referer != "" {
					span.SetAttributes(attribute.String("http.request.header.referer", referer))
				}
			}

			h.ServeHTTP(w, r)
		}), "",
			otelhttp.WithTracerProvider(provider),
			otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
				return fmt.Sprintf("%s %s", r.Method, r.URL.Path)
			}),
			otelhttp.WithFilter(httpFilter),
			otelhttp.WithPropagators(SafePropagators()),
		)
	})

	global.mu.Lock()
	global.tracerProvider = provider
	global.mu.Unlock()

	return nil
}

func consumeExporters() []sdktrace.SpanExporter {
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
	tp *sdktrace.TracerProvider
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

type fileExporter struct {
	sdktrace.SpanExporter
	file *os.File
}

func (f fileExporter) Shutdown(ctx context.Context) error {
	return errors.Join(f.SpanExporter.Shutdown(ctx), f.file.Close())
}

func Propagators() propagation.TextMapPropagator {
	return propagation.NewCompositeTextMapPropagator(NamespaceTraceParent{}, propagation.TraceContext{}, propagation.Baggage{})
}

func SafePropagators() propagation.TextMapPropagator {
	return propagation.NewCompositeTextMapPropagator(NamespaceTraceParent{})
}
