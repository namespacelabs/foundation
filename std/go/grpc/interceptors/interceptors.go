// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package interceptors

import (
	"context"
	"sync"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"google.golang.org/grpc"
	"google.golang.org/grpc/stats"
	"namespacelabs.dev/foundation/std/go/core"
)

var (
	interceptorsMu sync.RWMutex

	serverInterceptors struct {
		registrations []Registered
	}

	clientInterceptors struct {
		registrations []Registration // Each index of `unary` and `streaming` maps back to the same index `Registration`.
		unary         []grpc.UnaryClientInterceptor
		streaming     []grpc.StreamClientInterceptor
		handlers      []stats.Handler
	}
)

type Registration struct {
	owner *core.InstantiationPath
	name  string
	after []string
}

type Registered struct {
	Name    string
	After   []string
	Unary   grpc.UnaryServerInterceptor
	Stream  grpc.StreamServerInterceptor
	Handler stats.Handler
}

func (r Registration) ForClient(u grpc.UnaryClientInterceptor, s grpc.StreamClientInterceptor) {
	core.AssertNotRunning("AddClientInterceptor")

	interceptorsMu.Lock()
	defer interceptorsMu.Unlock()

	clientInterceptors.registrations = append(clientInterceptors.registrations, r)
	clientInterceptors.unary = append(clientInterceptors.unary, u)
	clientInterceptors.streaming = append(clientInterceptors.streaming, s)
}

func (r Registration) HandlerForClient(u stats.Handler) {
	core.AssertNotRunning("AddServerInterceptor")

	interceptorsMu.Lock()
	defer interceptorsMu.Unlock()

	clientInterceptors.registrations = append(clientInterceptors.registrations, r)
	clientInterceptors.handlers = append(clientInterceptors.handlers, u)
}

func (r Registration) ForServer(u grpc.UnaryServerInterceptor, s grpc.StreamServerInterceptor) {
	core.AssertNotRunning("AddServerInterceptor")

	interceptorsMu.Lock()
	defer interceptorsMu.Unlock()

	serverInterceptors.registrations = append(serverInterceptors.registrations, Registered{
		Name:   r.name,
		After:  r.after,
		Unary:  u,
		Stream: s,
	})
}

func (r Registration) HandlerForServer(u stats.Handler) {
	core.AssertNotRunning("AddServerInterceptor")

	interceptorsMu.Lock()
	defer interceptorsMu.Unlock()

	serverInterceptors.registrations = append(serverInterceptors.registrations, Registered{
		Name:    r.name,
		After:   r.after,
		Handler: u,
	})
}

func ServerInterceptors() []Registered {
	interceptorsMu.RLock()
	defer interceptorsMu.RUnlock()
	return serverInterceptors.registrations
}

func ClientInterceptors() []grpc.DialOption {
	interceptorsMu.RLock()
	defer interceptorsMu.RUnlock()

	var opts []grpc.DialOption

	for _, h := range clientInterceptors.handlers {
		opts = append(opts, grpc.WithStatsHandler(h))
	}

	opts = append(opts,
		grpc.WithStreamInterceptor(grpc_middleware.ChainStreamClient(clientInterceptors.streaming...)),
		grpc.WithUnaryInterceptor(grpc_middleware.ChainUnaryClient(clientInterceptors.unary...)),
	)

	return opts
}

func ProvideInterceptorRegistration(ctx context.Context, r *InterceptorRegistration) (Registration, error) {
	return Registration{owner: core.InstantiationPathFromContext(ctx), name: r.GetName(), after: r.GetAfter()}, nil
}
