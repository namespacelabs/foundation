// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package server

import (
	"context"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"namespacelabs.dev/foundation/std/go/core"
)

type Registrar interface {
	grpc.ServiceRegistrar

	Handle(path string, p http.Handler) *mux.Route
	PathPrefix(path string) *mux.Route
}

type Server interface {
	InternalRegisterGrpcGateway(reg func(context.Context, *runtime.ServeMux, *grpc.ClientConn) error)
	Scope(*core.Package) Registrar
}

var _ Server = &ServerImpl{}

// Implements the grpc.ServiceRegistrar interface.
type ServerImpl struct {
	srv     *grpc.Server
	httpMux *mux.Router

	gatewayRegistrations []func(context.Context, *runtime.ServeMux, *grpc.ClientConn) error
}

func (s *ServerImpl) RegisterService(desc *grpc.ServiceDesc, impl interface{}) {
	s.srv.RegisterService(desc, impl)
}

func (s *ServerImpl) HandleFunc(path string, f func(http.ResponseWriter, *http.Request)) *mux.Route {
	return s.Handle(path, http.HandlerFunc(f))
}

func (s *ServerImpl) InternalRegisterGrpcGateway(reg func(context.Context, *runtime.ServeMux, *grpc.ClientConn) error) {
	s.gatewayRegistrations = append(s.gatewayRegistrations, reg)
}

// Deprecated; should use InternalRegisterGrpcGateway.
func (g *ServerImpl) RegisterGrpcGateway(reg func(context.Context, *runtime.ServeMux, *grpc.ClientConn) error) {
	g.InternalRegisterGrpcGateway(reg)
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
