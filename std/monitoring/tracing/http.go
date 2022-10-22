// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package tracing

import (
	"context"
	"fmt"
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/trace"
)

type HttpClientProvider struct {
	provider trace.TracerProvider
}

func (hp HttpClientProvider) Wrap(client *http.Client) *http.Client {
	client.Transport = otelhttp.NewTransport(client.Transport,
		otelhttp.WithTracerProvider(hp.provider),
		otelhttp.WithSpanNameFormatter(func(operation string, r *http.Request) string {
			return fmt.Sprintf("HTTP %s %s%s", r.Method, r.URL.Host, r.URL.Path)
		}))
	return client
}

func ProvideHttpClientProvider(ctx context.Context, _ *NoArgs, deps ExtensionDeps) (HttpClientProvider, error) {
	provider, err := getTracerProvider()
	if err != nil {
		return HttpClientProvider{}, err
	}

	return HttpClientProvider{provider}, nil
}
