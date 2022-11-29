// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package server

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
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
	"namespacelabs.dev/foundation/std/go/grpc/interceptors"
	"namespacelabs.dev/foundation/std/go/http/middleware"
	"namespacelabs.dev/foundation/std/grpc/requestid"
)

var (
	listenHostname = flag.String("listen_hostname", "localhost", "Hostname to listen on.")
	port           = flag.Int("port", 0, "Port to listen on.")
	httpPort       = flag.Int("http_port", 0, "Port to listen HTTP on.")

	handleSIGTERM = false
)

const (
	developmentDrainTimeout = 5 * time.Second
	productionDrainTimeout  = 30 * time.Second
)

func ListenPort() int { return *port }

func InitializationDone() {
	core.InitializationDone()
}

func Listen(ctx context.Context, registerServices func(Server)) error {
	if handleSIGTERM {
		go func() {
			sigint := make(chan os.Signal, 1)

			signal.Notify(sigint, os.Interrupt)
			signal.Notify(sigint, syscall.SIGTERM)

			r2 := <-sigint

			log.Printf("got %v", r2)

			// XXX support more graceful shutdown. Although
			// https://github.com/kubernetes/kubernetes/issues/86280#issuecomment-583173036
			// "What you SHOULD do is hear the SIGTERM and start wrapping up. What
			// you should NOT do is close your listening socket. If you win the
			// race, you will receive traffic and reject it.""

			// So we start failing readiness, so we're removed from the serving set.
			// Then we wait for a bit for traffic to drain out. And then we leave.

			core.MarkShutdownStarted()

			if core.EnvIs(schema.Environment_DEVELOPMENT) {
				// In development, we drain quickly.
				time.Sleep(developmentDrainTimeout)
			} else {
				time.Sleep(productionDrainTimeout)
			}

			if r2 == syscall.SIGTERM {
				os.Exit(0)
			} else {
				os.Exit(1)
			}
		}()
	}

	lis, err := net.Listen("tcp", fmt.Sprintf("%s:%d", *listenHostname, *port))
	if err != nil {
		return err
	}

	m := cmux.New(lis)

	httpL := m.Match(cmux.HTTP1())
	anyL := m.Match(cmux.Any())

	opts := interceptorsAsOpts()

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

		opts = append(opts, grpc.Creds(transportCreds))
	}

	grpcServer := grpc.NewServer(opts...)

	// Enable tooling to query which gRPC services, etc are exported by this server.
	reflection.Register(grpcServer)

	httpMux := mux.NewRouter()
	httpMux.Use(middleware.Consume()...)
	httpMux.Use(proxyHeaders)
	httpMux.Use(func(h http.Handler) http.Handler {
		return handlers.CombinedLoggingHandler(os.Stdout, h)
	})

	s := &ServerImpl{srv: grpcServer, httpMux: httpMux}
	registerServices(s)

	// Export standard metrics.
	grpc_prometheus.Register(grpcServer)

	// XXX keep track of per-service health.
	grpc_health_v1.RegisterHealthServer(grpcServer, health.NewServer())

	// XXX configurable logging.
	core.Log.Printf("Starting to listen on %v", lis.Addr())

	// Set runtime.GOMAXPROCS to respect container limits if the env var GOMAXPROCS is not set or is invalid, preventing CPU throttling.
	if _, err := maxprocs.Set(maxprocs.Logger(core.Log.Printf)); err != nil {
		core.Log.Printf("Failed to reset GOMAXPROCS: %v", err)
	}

	debugMux := mux.NewRouter()
	core.RegisterDebugEndpoints(debugMux)

	debugHTTP := &http.Server{Handler: debugMux}
	go func() { checkReturn("http/debug", debugHTTP.Serve(httpL)) }()
	go func() { checkReturn("grpc", grpcServer.Serve(anyL)) }()

	if *httpPort != 0 {
		httpServer := &http.Server{Handler: httpMux}

		gwLis, err := net.Listen("tcp", fmt.Sprintf("%s:%d", *listenHostname, *httpPort))
		if err != nil {
			return err
		}

		core.Log.Printf("Starting HTTP listen on %v", gwLis.Addr())

		go func() { checkReturn("http", httpServer.Serve(gwLis)) }()
	}

	return m.Serve()
}

func interceptorsAsOpts() []grpc.ServerOption {
	unary, streaming := interceptors.ServerInterceptors()

	var coreU []grpc.UnaryServerInterceptor
	var coreS []grpc.StreamServerInterceptor

	// Interceptors are always invoked in order. It's **imperative** that the
	// request id handling interceptor shows up first.

	coreU = append(coreU, requestid.Interceptor{}.Unary)
	coreS = append(coreS, requestid.Interceptor{}.Streaming)

	coreU = append(coreU, unary...)
	coreS = append(coreS, streaming...)

	return []grpc.ServerOption{
		grpc.StreamInterceptor(grpc_middleware.ChainStreamServer(coreS...)),
		grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(coreU...)),
	}
}

func checkReturn(what string, err error) {
	if err != nil {
		core.Log.Fatalf("%s: serving failed: %v", what, err)
	}
}
