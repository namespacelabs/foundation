// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package honeycomb

import (
	"context"
	"os"

	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"google.golang.org/grpc/credentials"
)

func Create(ctx context.Context, key string) (*otlptrace.Exporter, error) {
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

func Prepare(ctx context.Context, deps ExtensionDeps) error {
	xHoneycombTeam := os.Getenv("MONITORING_HONEYCOMB_X_HONEYCOMB_TEAM")
	if xHoneycombTeam == "" {
		// No secret specified.
		return nil
	}

	exporter, err := Create(ctx, xHoneycombTeam)
	if err != nil {
		return err
	}

	return deps.OpenTelemetry.Register(exporter)
}
