// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package servercore

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"slices"
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
	"namespacelabs.dev/foundation/framework/runtime"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/go/core"
	gogrpc "namespacelabs.dev/foundation/std/go/grpc"
	"namespacelabs.dev/foundation/std/go/http/middleware"
	"namespacelabs.dev/foundation/std/grpc/requestid"
)

var tlsPort = flag.String("grpcserver_multiplex_tls", "", "Multiplex TLS connections from the default listener to this named listener.")

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
	CreateListener      func(context.Context) (net.Listener, error)
	CreateNamedListener func(context.Context, string) (net.Listener, error)
	CreateHttpListener  func(context.Context) (net.Listener, HTTPOptions, error)

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

	var tlsL net.Listener = nil
	if *tlsPort != "" {
		tlsL = m.Match(cmux.TLS())
	}

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

	defaultServer := grpc.NewServer(grpcopts...)
	serversByConfiguration := map[string]*grpc.Server{
		"": defaultServer,
	}

	rt, err := runtime.LoadRuntimeConfig()
	if err != nil {
		return err
	}

	listeners := map[string]net.Listener{}
	for _, cfg := range rt.ListenerConfiguration {
		c := listenerConfiguration(cfg.Name)
		if c == nil {
			return fnerrors.New("missing listener configuration for %q", cfg.Name)
		}

		switch cfg.Protocol {
		case "grpc":
			if cgrp, ok := c.(GrpcListenerConfiguration); ok {
				x := append(slices.Clone(grpcopts), cgrp.ServerOpts(cfg.Name)...)
				if creds := cgrp.TransportCredentials(cfg.Name); creds != nil {
					x = append(x, grpc.Creds(creds))
				}
				serversByConfiguration[cfg.Name] = grpc.NewServer(x...)
			} else {
				return fnerrors.New("listener configuration for %q does not support grpc", cfg.Name)
			}

		case "":
			lst, err := c.CreateListener(ctx, cfg.Name, opts)
			if err != nil {
				return err
			}

			listeners[cfg.Name] = lst

		default:
			return fnerrors.New("unsupported service protocol %q", cfg.Protocol)
		}
	}

	if core.EnvPurpose() != schema.Environment_PRODUCTION {
		// Enable tooling to query which gRPC services, etc are exported by this server.
		reflection.Register(defaultServer)
	}

	httpMux := NewHTTPMux(middleware.Registered()...)

	s := &ServerImpl{srv: serversByConfiguration, httpMux: httpMux}
	registerServices(s)

	grpc_prometheus.EnableHandlingTimeHistogram()

	// Export standard metrics.
	for _, srv := range serversByConfiguration {
		grpc_prometheus.Register(srv)
		grpc_health_v1.RegisterHealthServer(srv, health.NewServer())
	}

	// XXX keep track of per-service health.

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

		httpServer = NewHttp2CapableServer(httpMux, opts)

		core.ZLog.Info().Msgf("Starting HTTP listen on %v", gwLis.Addr())

		eg.Go(func() error { return ListenAndGracefullyShutdownHTTP(egCtx, "http", httpServer, gwLis) })
	}

	debugHTTP := &http.Server{Handler: debugMux}
	eg.Go(func() error { return ListenAndGracefullyShutdownHTTP(egCtx, "http/debug", debugHTTP, httpL) })

	eg.Go(func() error { return ListenAndGracefullyShutdownGRPC(egCtx, "grpc", defaultServer, anyL) })

	for k, srv := range serversByConfiguration {
		k := k     // Close k.
		srv := srv // Close srv.

		if k != "" {
			grpcLis, err := listenerConfiguration(k).CreateListener(ctx, k, opts)
			if err != nil {
				return err
			}

			core.ZLog.Info().Msgf("Starting configuration %q listen on %v", k, grpcLis.Addr())

			eg.Go(func() error { return ListenAndGracefullyShutdownGRPC(egCtx, "grpc-"+k, srv, grpcLis) })

			if k == *tlsPort {
				eg.Go(func() error { return ListenAndGracefullyShutdownGRPC(egCtx, "grpc/tls", srv, tlsL) })
			}
		}
	}

	eg.Go(func() error { return ignoreClosure("grpc", m.Serve()) })

	for _, reg := range s.listeners {
		lst := listeners[reg.ConfigurationName]
		if lst == nil {
			return fnerrors.New("%q registered for a listener with %q, but there's none", reg.PackageName, reg.ConfigurationName)
		}

		reg := reg // Close reg.
		eg.Go(func() error { return reg.Handler(egCtx, lst) })
	}

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

func NewHttp2CapableServer(mux http.Handler, opts HTTPOptions) *http.Server {
	return &http.Server{
		Handler: h2c.NewHandler(mux, &http2.Server{
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
}

func ListenAndGracefullyShutdownGRPC(ctx context.Context, label string, srv *grpc.Server, lis net.Listener) error {
	return listenAndGracefullyShutdown(ctx, label, func() error {
		return srv.Serve(lis)
	}, func() error {
		srv.GracefulStop()
		return nil
	})
}

func ListenAndGracefullyShutdownHTTP(ctx context.Context, label string, srv *http.Server, lis net.Listener) error {
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
