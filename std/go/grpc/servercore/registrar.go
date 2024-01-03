// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package servercore

import (
	"net/http"

	"github.com/gorilla/mux"
	"google.golang.org/grpc"
	"namespacelabs.dev/foundation/std/go/core"
)

type Registrar interface {
	grpc.ServiceRegistrar

	Handle(path string, p http.Handler) *mux.Route
	PathPrefix(path string) *mux.Route
}

type Server interface {
	Scope(*core.Package) Registrar
}

var _ Server = &ServerImpl{}

// Implements the grpc.ServiceRegistrar interface.
type ServerImpl struct {
	srv     *grpc.Server
	httpMux *mux.Router
}

func (s *ServerImpl) RegisterService(desc *grpc.ServiceDesc, impl interface{}) {
	s.srv.RegisterService(desc, impl)
}

func (s *ServerImpl) HandleFunc(path string, f func(http.ResponseWriter, *http.Request)) *mux.Route {
	return s.Handle(path, http.HandlerFunc(f))
}

func (s *ServerImpl) Handle(path string, p http.Handler) *mux.Route {
	return s.httpMux.Handle(path, p)
}

func (s *ServerImpl) PathPrefix(path string) *mux.Route {
	return s.httpMux.PathPrefix(path)
}

func (s *ServerImpl) Scope(pkg *core.Package) Registrar {
	return &scopedServer{parent: s, pkg: pkg.PackageName}
}

type scopedServer struct {
	pkg    string
	parent *ServerImpl
}

func (s *scopedServer) RegisterService(desc *grpc.ServiceDesc, impl interface{}) {
	s.parent.RegisterService(desc, impl)
}

func (s *scopedServer) Handle(path string, p http.Handler) *mux.Route {
	return s.parent.Handle(path, p)
}

func (s *scopedServer) PathPrefix(path string) *mux.Route {
	return s.parent.PathPrefix(path)
}

func proxyHeaders(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if forwardedFor := r.Header.Get("X-Forwarded-For"); forwardedFor != "" {
			r.RemoteAddr = forwardedFor
		}

		if host := r.Header.Get("X-Forwarded-Host"); host != "" {
			r.URL.Host = host

			// Set the scheme (proto) with the value passed from the proxy.
			// But only do so if there's a host present.
			if scheme := r.Header.Get("X-Forwarded-Scheme"); scheme != "" {
				r.URL.Scheme = scheme
			}
		}

		// Call the next handler in the chain.
		h.ServeHTTP(w, r)
	})
}
