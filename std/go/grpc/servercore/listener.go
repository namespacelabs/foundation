// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package servercore

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/rs/zerolog/hlog"
	"github.com/soheilhy/cmux"
	"go.uber.org/automaxprocs/maxprocs"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/go/core"
	gogrpc "namespacelabs.dev/foundation/std/go/grpc"
	"namespacelabs.dev/foundation/std/go/http/middleware"
	"namespacelabs.dev/foundation/std/grpc/requestid"
)

type HTTPOptions struct {
	HTTP1ReadTimeout       time.Duration `json:"http_read_timeout"`
	HTTP1ReadHeaderTimeout time.Duration `json:"http_read_header_timeout"`
	HTTP1WriteTimeout      time.Duration `json:"http_write_timeout"`
	HTTP1IdleTimeout       time.Duration `json:"http_idle_timeout"`
	HTTP1MaxHeaderBytes    int           `json:"http_max_header_bytes"`

	HTTP2MaxConcurrentStreams         uint32        `json:"http2_max_concurrent_streams"`
	HTTP2MaxReadFrameSize             uint32        `json:"http2_max_read_frame_size"`
	HTTP2IdleTimeout                  time.Duration `json:"http2_idle_timeout"`
	HTTP2MaxUploadBufferPerConnection int32         `json:"http2_max_upload_buffer_per_connection"`
	HTTP2MaxUploadBufferPerStream     int32         `json:"http2_max_upload_buffer_per_stream"`
}

type ListenOpts struct {
	CreateListener     func(context.Context) (net.Listener, error)
	CreateHttpListener func(context.Context) (net.Listener, HTTPOptions, error)

	DontHandleSigTerm bool
}

func MakeTCPListener(address string, port int) func(context.Context) (net.Listener, error) {
	return func(ctx context.Context) (net.Listener, error) {
		return net.Listen("tcp", fmt.Sprintf("%s:%d", address, port))
	}
}

func NewHTTPMux(middleware ...mux.MiddlewareFunc) *mux.Router {
	httpMux := mux.NewRouter()

	httpMux.Use(proxyHeaders)
	httpMux.Use(middleware...)

	httpMux.Use(func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, rdata := requestid.AllocateRequestID(r.Context())

			log := core.ZLog.With().Str("ns.rid", string(rdata.RequestID))
			logger := log.Logger()

			h.ServeHTTP(w, r.WithContext(logger.WithContext(ctx)))
		})
	})

	httpMux.Use(hlog.AccessHandler(func(r *http.Request, status, size int, duration time.Duration) {
		hlog.FromRequest(r).Info().
			Str("method", r.Method).
			Stringer("url", r.URL).
			Int("status", status).
			Int("size", size).
			Dur("duration", duration).
			Send()
	}))

	return httpMux
}

func Listen(ctx context.Context, opts ListenOpts, registerServices func(Server)) error {
	if !opts.DontHandleSigTerm {
		go handleGracefulShutdown()
	}

	lis, err := opts.CreateListener(ctx)
	if err != nil {
		return err
	}

	m := cmux.New(lis)

	httpL := m.Match(cmux.HTTP1())
	anyL := m.Match(cmux.Any())

	grpcopts := OrderedServerInterceptors()
	if gogrpc.ServerCreds != nil {
		grpcopts = append(grpcopts, grpc.Creds(gogrpc.ServerCreds))
	}

	grpcopts = append(grpcopts, grpc.KeepaliveParams(keepalive.ServerParameters{
		// Without keepalives Nginx-ingress gives up on long-running streaming RPCs.
		Time: 30 * time.Second,
	}))

	grpcServer := grpc.NewServer(grpcopts...)

	if core.EnvPurpose() != schema.Environment_PRODUCTION {
		// Enable tooling to query which gRPC services, etc are exported by this server.
		reflection.Register(grpcServer)
	}

	httpMux := NewHTTPMux(middleware.Consume()...)

	s := &ServerImpl{srv: grpcServer, httpMux: httpMux}
	registerServices(s)

	// Export standard metrics.
	grpc_prometheus.Register(grpcServer)
	grpc_prometheus.EnableHandlingTimeHistogram()

	// XXX keep track of per-service health.
	grpc_health_v1.RegisterHealthServer(grpcServer, health.NewServer())

	// XXX configurable logging.
	core.ZLog.Info().Msgf("Starting to listen on %v", lis.Addr())

	// Set runtime.GOMAXPROCS to respect container limits if the env var GOMAXPROCS is not set or is invalid, preventing CPU throttling.
	if _, err := maxprocs.Set(maxprocs.Logger(core.ZLog.Printf)); err != nil {
		core.ZLog.Debug().Msgf("Failed to reset GOMAXPROCS: %v", err)
	}

	debugMux := mux.NewRouter()
	core.RegisterDebugEndpoints(debugMux)

	debugHTTP := &http.Server{Handler: debugMux}
	go func() { checkReturn("http/debug", debugHTTP.Serve(httpL)) }()
	go func() { checkReturn("grpc", grpcServer.Serve(anyL)) }()

	if opts.CreateHttpListener != nil {
		gwLis, opts, err := opts.CreateHttpListener(ctx)
		if err != nil {
			return err
		}

		httpServer := &http.Server{
			Handler: h2c.NewHandler(httpMux, &http2.Server{
				MaxConcurrentStreams:         opts.HTTP2MaxConcurrentStreams,
				MaxReadFrameSize:             opts.HTTP2MaxReadFrameSize,
				IdleTimeout:                  opts.HTTP2IdleTimeout,
				MaxUploadBufferPerConnection: opts.HTTP2MaxUploadBufferPerConnection,
				MaxUploadBufferPerStream:     opts.HTTP2MaxUploadBufferPerStream,
			}),
			ReadTimeout:       opts.HTTP1ReadTimeout,
			ReadHeaderTimeout: opts.HTTP1ReadHeaderTimeout,
			WriteTimeout:      opts.HTTP1WriteTimeout,
			IdleTimeout:       opts.HTTP1IdleTimeout,
			MaxHeaderBytes:    opts.HTTP1MaxHeaderBytes,
		}

		core.ZLog.Info().Msgf("Starting HTTP listen on %v", gwLis.Addr())

		go func() { checkReturn("http", httpServer.Serve(gwLis)) }()
	}

	return m.Serve()
}

func checkReturn(what string, err error) {
	if err != nil {
		core.ZLog.Fatal().Err(err).Str("what", what).Msg("serving failed")
	}
}
