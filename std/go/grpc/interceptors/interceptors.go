// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package interceptors

import (
	"context"
	"sync"

	"google.golang.org/grpc"
	"namespacelabs.dev/foundation/std/go/core"
)

var (
	interceptorsMu sync.RWMutex

	serverInterceptors struct {
		registrations []Registration // Each index of `unary` and `streaming` maps back to the same index `Registration`.
		unary         []grpc.UnaryServerInterceptor
		streaming     []grpc.StreamServerInterceptor
	}

	clientInterceptors struct {
		registrations []Registration // Each index of `unary` and `streaming` maps back to the same index `Registration`.
		unary         []grpc.UnaryClientInterceptor
		streaming     []grpc.StreamClientInterceptor
	}
)

type Registration struct {
	owner *core.InstantiationPath
	name  string
}

func (r Registration) ForClient(u grpc.UnaryClientInterceptor, s grpc.StreamClientInterceptor) {
	core.AssertNotRunning("AddServerInterceptor")

	interceptorsMu.Lock()
	defer interceptorsMu.Unlock()

	clientInterceptors.registrations = append(clientInterceptors.registrations, r)
	clientInterceptors.unary = append(clientInterceptors.unary, u)
	clientInterceptors.streaming = append(clientInterceptors.streaming, s)
}

func (r Registration) ForServer(u grpc.UnaryServerInterceptor, s grpc.StreamServerInterceptor) {
	core.AssertNotRunning("AddServerInterceptor")

	interceptorsMu.Lock()
	defer interceptorsMu.Unlock()

	serverInterceptors.registrations = append(serverInterceptors.registrations, r)
	serverInterceptors.unary = append(serverInterceptors.unary, u)
	serverInterceptors.streaming = append(serverInterceptors.streaming, s)
}

func ServerInterceptors() ([]grpc.UnaryServerInterceptor, []grpc.StreamServerInterceptor) {
	interceptorsMu.RLock()
	defer interceptorsMu.RUnlock()

	unary := serverInterceptors.unary
	streaming := serverInterceptors.streaming
	return unary, streaming
}

func ClientInterceptors() ([]grpc.UnaryClientInterceptor, []grpc.StreamClientInterceptor) {
	interceptorsMu.RLock()
	defer interceptorsMu.RUnlock()

	unary := clientInterceptors.unary
	streaming := clientInterceptors.streaming
	return unary, streaming
}

func ProvideInterceptorRegistration(ctx context.Context, r *InterceptorRegistration) (Registration, error) {
	return Registration{owner: core.InstantiationPathFromContext(ctx), name: r.GetName()}, nil
}
