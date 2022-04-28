// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package tracing

import (
	"context"
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/trace"
)

type HttpClientProvider struct {
	provider trace.TracerProvider
}

func (hp HttpClientProvider) Wrap(client *http.Client) *http.Client {
	client.Transport = otelhttp.NewTransport(client.Transport, otelhttp.WithTracerProvider(hp.provider))
	return client
}

func ProvideHttpClientProvider(ctx context.Context, _ *NoArgs, deps ExtensionDeps) (HttpClientProvider, error) {
	provider, err := getTracerProvider()
	if err != nil {
		return HttpClientProvider{}, err
	}

	return HttpClientProvider{provider}, nil
}
