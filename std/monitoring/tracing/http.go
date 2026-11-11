// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package tracing

import (
	"context"
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type HttpClientProvider struct {
	provider trace.TracerProvider
}

func (hp HttpClientProvider) New() *http.Client {
	return hp.Wrap(&http.Client{})
}

func (hp HttpClientProvider) Wrap(client *http.Client) *http.Client {
	client.Transport = hp.WrapTransport(client.Transport)
	return client
}

func (hp HttpClientProvider) WrapTransport(base http.RoundTripper, options ...otelhttp.Option) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return httpClientTransport{otelhttp.NewTransport(httpClientErrorTransport{base}, append([]otelhttp.Option{otelhttp.WithTracerProvider(hp.provider), otelhttp.WithPropagators(Propagators())}, options...)...)}
}

type httpClientTransport struct {
	base http.RoundTripper
}

func (t httpClientTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	// Isolate classification from incoming server requests and concurrent or
	// retried client requests that share their context's OTel labeler.
	labeler := &otelhttp.Labeler{}
	if inherited, ok := otelhttp.LabelerFromContext(r.Context()); ok {
		labeler.Add(inherited.Get()...)
	}
	return t.base.RoundTrip(r.WithContext(otelhttp.ContextWithLabeler(r.Context(), labeler)))
}

type httpClientErrorTransport struct {
	base http.RoundTripper
}

func (t httpClientErrorTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	res, err := t.base.RoundTrip(r)
	labeler, _ := otelhttp.LabelerFromContext(r.Context())
	// OTel's concrete error.type cannot distinguish context.Canceled from
	// unrelated errors.New errors. Check the request context so custom cancel
	// causes are recognized without classifying deadlines or HTTP responses.
	labeler.Add(attribute.Bool("http.request.canceled", err != nil && r.Context().Err() == context.Canceled))
	return res, err
}

func ProvideHttpClientProvider(ctx context.Context, _ *NoArgs, deps ExtensionDeps) (HttpClientProvider, error) {
	provider, err := getTracerProvider()
	if err != nil {
		return HttpClientProvider{}, err
	}

	return HttpClientProvider{provider}, nil
}
