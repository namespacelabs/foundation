// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package servercore

import (
	"context"
	"crypto/tls"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
)

func TestMarkPlaintextTransport(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	ctx, span := provider.Tracer("test").Start(context.Background(), "rpc")

	markPlaintextTransport(peer.NewContext(ctx, &peer.Peer{}))
	span.End()

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("got %d spans, want 1", len(spans))
	}
	for _, attr := range spans[0].Attributes {
		if string(attr.Key) == grpcTransportSecurityAttribute && attr.Value.AsString() == "plaintext" {
			return
		}
	}
	t.Fatal("plaintext transport attribute is missing")
}

func TestMarkPlaintextTransportDoesNotMarkTLS(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	ctx, span := provider.Tracer("test").Start(context.Background(), "rpc")
	ctx = peer.NewContext(ctx, &peer.Peer{AuthInfo: credentials.TLSInfo{State: tls.ConnectionState{}}})

	markPlaintextTransport(ctx)
	span.End()

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("got %d spans, want 1", len(spans))
	}
	for _, attr := range spans[0].Attributes {
		if string(attr.Key) == grpcTransportSecurityAttribute {
			t.Fatalf("unexpected transport security attribute: %v", attr)
		}
	}
}
