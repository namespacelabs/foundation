// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package server

import (
	"context"
	"net/http"
	"os"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
)

// Implements the grpc.ServiceRegistrar interface.
type Grpc struct {
	srv     *grpc.Server
	httpMux *mux.Router

	gatewayRegistrations []func(context.Context, *runtime.ServeMux, *grpc.ClientConn) error
}

func (s *Grpc) RegisterService(desc *grpc.ServiceDesc, impl interface{}) {
	s.srv.RegisterService(desc, impl)
}

func (s *Grpc) HandleFunc(path string, f func(http.ResponseWriter, *http.Request)) *mux.Route {
	return s.Handle(path, http.HandlerFunc(f))
}

func (s *Grpc) RegisterGrpcGateway(reg func(context.Context, *runtime.ServeMux, *grpc.ClientConn) error) {
	s.gatewayRegistrations = append(s.gatewayRegistrations, reg)
}

func MakeHandler(p http.Handler) http.Handler {
	p = handlers.LoggingHandler(os.Stdout, p)
	p = proxyHeaders(p) // We always run behind a reverse proxy.
	return p
}

func (s *Grpc) Handle(path string, p http.Handler) *mux.Route {
	return s.httpMux.Handle(path, MakeHandler(p))
}

func (s *Grpc) PathPrefix(path string) *mux.Route {
	return s.httpMux.PathPrefix(path)
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