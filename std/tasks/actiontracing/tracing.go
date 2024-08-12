// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package actiontracing

import (
	"context"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/credentials"
)

var Tracer trace.Tracer

func SetupJaegerTracing(ctx context.Context, jaegerEndpoint string) (context.Context, func()) {
	tp, err := createTracer(jaegerEndpoint)
	if err != nil {
		panic(err)
	}

	return SetupTracing(ctx, tp)
}

func SetupTracing(ctx context.Context, tp *tracesdk.TracerProvider) (context.Context, func()) {
	// Register our TracerProvider as the global so any imported
	// instrumentation in the future will default to using it.
	otel.SetTracerProvider(tp)

	Tracer = tp.Tracer("foundation")

	spanCtx, span := Tracer.Start(ctx, "ns (cli invocation)")

	return spanCtx, func() {
		span.End()

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()
		_ = tp.Shutdown(ctx)
	}
}

func createTracer(url string) (*tracesdk.TracerProvider, error) {
	exp, err := jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(url)))
	if err != nil {
		return nil, err
	}

	return CreateTracerForExporter(exp), nil
}

func CreateTracerForExporter(exp tracesdk.SpanExporter) *tracesdk.TracerProvider {
	attrs := []attribute.KeyValue{
		semconv.ServiceName("foundation"),
	}

	if env := os.Getenv("FOUNDATION_TRACING_ENVIRONMENT"); env != "" {
		attrs = append(attrs, semconv.DeploymentEnvironment(env))
	}

	return tracesdk.NewTracerProvider(
		tracesdk.WithBatcher(exp, tracesdk.WithMaxExportBatchSize(1)),
		tracesdk.WithResource(resource.NewWithAttributes(semconv.SchemaURL, attrs...)),
	)
}

func CreateHoneycombExporter(ctx context.Context, key string) (*otlptrace.Exporter, error) {
	opts := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint("api.honeycomb.io:443"),
		otlptracegrpc.WithHeaders(map[string]string{
			"x-honeycomb-team": key,
		}),
		otlptracegrpc.WithTLSCredentials(credentials.NewClientTLSFromCert(nil, "")),
	}

	client := otlptracegrpc.NewClient(opts...)
	return otlptrace.New(ctx, client)
}
