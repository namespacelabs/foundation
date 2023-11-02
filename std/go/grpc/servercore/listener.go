// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package servercore

import (
	"context"
	"errors"
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
	"golang.org/x/sync/errgroup"
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

	cancelCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	eg, egCtx := errgroup.WithContext(cancelCtx)

	var httpServer *http.Server
	if opts.CreateHttpListener != nil {
		gwLis, opts, err := opts.CreateHttpListener(ctx)
		if err != nil {
			return err
		}

		httpServer = &http.Server{
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

		eg.Go(func() error { return listenAndGracefullyShutdownHTTP(egCtx, "http", httpServer, gwLis) })
	}

	debugHTTP := &http.Server{Handler: debugMux}
	eg.Go(func() error { return listenAndGracefullyShutdownHTTP(egCtx, "http/debug", debugHTTP, httpL) })

	eg.Go(func() error { return listenAndGracefullyShutdownGRPC(egCtx, "grpc", grpcServer, anyL) })

	eg.Go(func() error { return ignoreClosure("grpc", m.Serve()) })

	// In development, we skip graceful shutdowns for faster iteration cycles.
	if !opts.DontHandleSigTerm && !core.EnvIs(schema.Environment_DEVELOPMENT) {
		eg.Go(func() error {
			handleGracefulShutdown(egCtx, cancel)
			return nil
		})
	}

	err = eg.Wait()
	core.ZLog.Info().Err(err).Msg("stopped listening")
	return err
}

func listenAndGracefullyShutdownGRPC(ctx context.Context, label string, srv *grpc.Server, lis net.Listener) error {
	return listenAndGracefullyShutdown(ctx, label, func() error {
		return srv.Serve(lis)
	}, func() error {
		srv.GracefulStop()
		return nil
	})
}

func listenAndGracefullyShutdownHTTP(ctx context.Context, label string, srv *http.Server, lis net.Listener) error {
	return listenAndGracefullyShutdown(ctx, label, func() error {
		return srv.Serve(lis)
	}, func() error {
		// We get here once ctx is already cancelled.
		// So continue shutting down in the background.
		return srv.Shutdown(context.Background())
	})
}

// Starts listenSync(). Once ctx is canceled runs shutdownSync() and waits for it to finish.
func listenAndGracefullyShutdown(ctx context.Context, label string, listenSync func() error, shutdownSync func() error) error {
	eg, egCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		err := ignoreClosure(label, listenSync())
		if err != nil {
			core.ZLog.Error().Err(err).Str("what", label).Msg("serving failed")
		}
		return err
	})
	eg.Go(func() error {
		// When Shutdown is called, Serve, ListenAndServe, and ListenAndServeTLS
		// immediately return ErrServerClosed. Make sure the program doesn't exit
		// and waits instead for Shutdown to return.
		// (https://pkg.go.dev/net/http#Server.Shutdown)
		<-egCtx.Done()
		err := ignoreClosure(label, shutdownSync())
		if err != nil {
			core.ZLog.Error().Str("what", label).Err(err).Msg("failed to stop server")
		} else {
			core.ZLog.Info().Str("what", label).Msg("stopped server")
		}
		return err
	})
	return eg.Wait()
}

func ignoreClosure(what string, err error) error {
	if err == nil ||
		errors.Is(err, net.ErrClosed) ||
		errors.Is(err, http.ErrServerClosed) ||
		errors.Is(err, cmux.ErrServerClosed) ||
		errors.Is(err, cmux.ErrListenerClosed) {
		// normal closure
		return nil
	}
	return fmt.Errorf("%s: %w", what, err)
}
