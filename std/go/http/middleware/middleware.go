// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package middleware

import (
	"context"
	"sync"

	"github.com/gorilla/mux"
	"namespacelabs.dev/foundation/std/go/core"
)

var middlewares struct {
	mu            sync.Mutex
	registrations []Middleware // Each index of `unary` and `streaming` maps back to the same index `Registration`.
	middlewares   []mux.MiddlewareFunc
}

type Middleware struct {
	owner *core.InstantiationPath
	name  string
}

func (r Middleware) Add(middleware mux.MiddlewareFunc) {
	core.AssertNotRunning("AddMiddleware")

	middlewares.mu.Lock()
	defer middlewares.mu.Unlock()

	middlewares.registrations = append(middlewares.registrations, r)
	middlewares.middlewares = append(middlewares.middlewares, middleware)
}

func Consume() []mux.MiddlewareFunc {
	middlewares.mu.Lock()
	defer middlewares.mu.Unlock()

	unary := middlewares.middlewares
	middlewares.registrations = nil
	middlewares.middlewares = nil
	return unary
}

func ProvideMiddleware(ctx context.Context, r *MiddlewareRegistration) (Middleware, error) {
	return Middleware{owner: core.InstantiationPathFromContext(ctx), name: r.GetName()}, nil
}
