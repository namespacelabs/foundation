// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package servercore

import (
	"context"
	"net"
	"net/http"

	"github.com/gorilla/mux"
	"google.golang.org/grpc"
	"namespacelabs.dev/foundation/std/go/core"
)

type Registrar interface {
	grpc.ServiceRegistrar

	Handle(path string, p http.Handler) *mux.Route
	PathPrefix(path string) *mux.Route
	RegisterListener(func(context.Context, net.Listener) error)
}

type Server interface {
	Scope(*core.Package) Registrar
}

var _ Server = &ServerImpl{}

// Implements the grpc.ServiceRegistrar interface.
type ServerImpl struct {
	srv       map[string][]*grpc.Server // Key is configuration name; "" is the default.
	listeners []listenerRegistration
	httpMux   *mux.Router
}

type listenerRegistration struct {
	PackageName       string
	ConfigurationName string
	Handler           func(context.Context, net.Listener) error
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

func (s *ServerImpl) registerService(pkg, config string, desc *grpc.ServiceDesc, impl interface{}) {
	if srvs, ok := s.srv[config]; ok {
		core.ZLog.Info().Str("package_name", pkg).Str("configuration", config).Msgf("Registered %v", desc.ServiceName)

		for _, srv := range srvs {
			srv.RegisterService(desc, impl)
		}
	} else {
		core.ZLog.Fatal().Str("package_name", pkg).Msgf("servercore: no such configuration %q (registering %v)", config, desc.ServiceName)
	}
}

func (s *ServerImpl) registerListener(pkg, config string, handler func(context.Context, net.Listener) error) {
	s.listeners = append(s.listeners, listenerRegistration{
		PackageName:       pkg,
		ConfigurationName: config,
		Handler:           handler,
	})
}

func (s *ServerImpl) Scope(pkg *core.Package) Registrar {
	return &scopedServer{pkg: pkg.PackageName, configurationName: pkg.ListenerConfiguration, parent: s}
}

type scopedServer struct {
	pkg               string
	configurationName string
	parent            *ServerImpl
}

func (s *scopedServer) RegisterService(desc *grpc.ServiceDesc, impl interface{}) {
	s.parent.registerService(s.pkg, s.configurationName, desc, impl)
}

func (s *scopedServer) Handle(path string, p http.Handler) *mux.Route {
	return s.parent.Handle(path, p)
}

func (s *scopedServer) PathPrefix(path string) *mux.Route {
	return s.parent.PathPrefix(path)
}

func (s *scopedServer) RegisterListener(handler func(context.Context, net.Listener) error) {
	s.parent.registerListener(s.pkg, s.configurationName, handler)
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
