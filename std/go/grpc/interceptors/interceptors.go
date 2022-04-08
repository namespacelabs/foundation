// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package interceptors

import (
	"context"
	"sync"

	"google.golang.org/grpc"
	"namespacelabs.dev/foundation/std/go/core"
	fninit "namespacelabs.dev/foundation/std/go/core/init"
)

var interceptors struct {
	mu            sync.Mutex
	registrations []Registration // Each index of `unary` and `streaming` maps back to the same index `Registration`.
	unary         []grpc.UnaryServerInterceptor
	streaming     []grpc.StreamServerInterceptor
}

type Registration struct {
	caller fninit.Caller
	name   string
}

func (r Registration) Add(u grpc.UnaryServerInterceptor, s grpc.StreamServerInterceptor) {
	core.AssertNotRunning("AddServerInterceptor")

	interceptors.mu.Lock()
	defer interceptors.mu.Unlock()

	interceptors.registrations = append(interceptors.registrations, r)
	interceptors.unary = append(interceptors.unary, u)
	interceptors.streaming = append(interceptors.streaming, s)
}

func Consume() ([]grpc.UnaryServerInterceptor, []grpc.StreamServerInterceptor) {
	interceptors.mu.Lock()
	defer interceptors.mu.Unlock()

	unary := interceptors.unary
	streaming := interceptors.streaming
	interceptors.registrations = nil
	interceptors.unary = nil
	interceptors.streaming = nil
	return unary, streaming
}

func ProvideInterceptorRegistration(_ context.Context, caller fninit.Caller, r *InterceptorRegistration) (Registration, error) {
	return Registration{caller: caller, name: r.GetName()}, nil
}
