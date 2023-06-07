// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package servercore

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"os"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/soheilhy/cmux"
	"go.uber.org/automaxprocs/maxprocs"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/go/core"
	gogrpc "namespacelabs.dev/foundation/std/go/grpc"
	"namespacelabs.dev/foundation/std/go/http/middleware"
	"namespacelabs.dev/foundation/std/grpc/requestid"
)

type ListenOpts struct {
	Address  string
	GrpcPort int
	HttpPort int

	DontHandleSigTerm bool
}

func Listen(ctx context.Context, opts ListenOpts, registerServices func(Server)) error {
	if !opts.DontHandleSigTerm {
		go handleGracefulShutdown()
	}

	lis, err := net.Listen("tcp", fmt.Sprintf("%s:%d", opts.Address, opts.GrpcPort))
	if err != nil {
		return err
	}

	m := cmux.New(lis)

	httpL := m.Match(cmux.HTTP1())
	anyL := m.Match(cmux.Any())

	grpcopts := interceptorsAsOpts()

	if gogrpc.ServerCert != nil {
		cert, err := tls.X509KeyPair(gogrpc.ServerCert.CertificateBundle, gogrpc.ServerCert.PrivateKey)
		if err != nil {
			return err
		}

		config := &tls.Config{
			Certificates: []tls.Certificate{cert},
			ClientAuth:   tls.NoClientCert,
		}

		transportCreds := credentials.NewTLS(config)

		grpcopts = append(grpcopts, grpc.Creds(transportCreds))
	}

	grpcServer := grpc.NewServer(grpcopts...)

	if core.EnvPurpose() != schema.Environment_PRODUCTION {
		// Enable tooling to query which gRPC services, etc are exported by this server.
		reflection.Register(grpcServer)
	}

	httpMux := mux.NewRouter()
	httpMux.Use(func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, _ := requestid.AllocateRequestID(r.Context())

			h.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	httpMux.Use(middleware.Consume()...)
	httpMux.Use(proxyHeaders)
	httpMux.Use(func(h http.Handler) http.Handler {
		return handlers.CombinedLoggingHandler(os.Stdout, h)
	})

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

	if opts.HttpPort != 0 {
		httpServer := &http.Server{Handler: httpMux}

		gwLis, err := net.Listen("tcp", fmt.Sprintf("%s:%d", opts.Address, opts.HttpPort))
		if err != nil {
			return err
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
