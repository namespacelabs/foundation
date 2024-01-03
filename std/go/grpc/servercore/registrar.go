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
	srv     map[string]*grpc.Server // Key is configuration name; "" is the default.
	httpMux *mux.Router
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

func (s *ServerImpl) RegisterServiceAtConfiguration(config string, desc *grpc.ServiceDesc, impl interface{}) {
	if srv, ok := s.srv[config]; ok {
		core.ZLog.Info().Str("configuration", config).Msgf("Registered %v", desc.ServiceName)
		srv.RegisterService(desc, impl)
	} else {
		core.ZLog.Fatal().Msgf("servercore: no such configuration %q (registering %v)", config, desc.ServiceName)
	}
}

func (s *ServerImpl) Scope(pkg *core.Package) Registrar {
	return &scopedServer{parent: s, configurationName: pkg.ConfigurationName}
}

type scopedServer struct {
	configurationName string
	parent            *ServerImpl
}

func (s *scopedServer) RegisterService(desc *grpc.ServiceDesc, impl interface{}) {
	s.parent.RegisterServiceAtConfiguration(s.configurationName, desc, impl)
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
