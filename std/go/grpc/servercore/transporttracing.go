// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package servercore

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
)

const grpcTransportSecurityAttribute = "namespace.grpc.transport_security"

func tracePlaintextUnaryServerInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	markPlaintextTransport(ctx)
	return handler(ctx, req)
}

func tracePlaintextStreamServerInterceptor(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	markPlaintextTransport(stream.Context())
	return handler(srv, stream)
}

func markPlaintextTransport(ctx context.Context) {
	p, ok := peer.FromContext(ctx)
	if !ok || p.AuthInfo != nil {
		return
	}

	trace.SpanFromContext(ctx).SetAttributes(attribute.String(grpcTransportSecurityAttribute, "plaintext"))
}
