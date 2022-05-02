// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package server

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/soheilhy/cmux"
	"go.uber.org/automaxprocs/maxprocs"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
	"namespacelabs.dev/foundation/std/go/core"
	"namespacelabs.dev/foundation/std/go/grpc/client"
	"namespacelabs.dev/foundation/std/go/grpc/interceptors"
	"namespacelabs.dev/foundation/std/go/http/middleware"
)

var (
	listenHostname = flag.String("listen_hostname", "localhost", "Hostname to listen on.")
	port           = flag.Int("port", 0, "Port to listen on.")
	httpPort       = flag.Int("http_port", 0, "Port to listen HTTP on.")
	gatewayPort    = flag.Int("gateway_port", 0, "Port to listen gRPC Gateway on.")
)

func ListenPort() int { return *port }

func InitializationDone() {
	core.InitializationDone()
}

func Listen(ctx context.Context, registerServices func(Server)) error {
	lis, err := net.Listen("tcp", fmt.Sprintf("%s:%d", *listenHostname, *port))
	if err != nil {
		return err
	}

	m := cmux.New(lis)
	httpL := m.Match(cmux.HTTP1())
	anyL := m.Match(cmux.Any())

	opts := interceptorsAsOpts()

	// XXX serving keys.
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
	go func() { listen("http/debug", debugHTTP.Serve(httpL)) }()
	go func() { listen("grpc", grpcServer.Serve(anyL)) }()

	if *httpPort != 0 {
		httpServer := &http.Server{Handler: httpMux}

		gwLis, err := net.Listen("tcp", fmt.Sprintf("%s:%d", *listenHostname, *httpPort))
		if err != nil {
			return err
		}

		core.Log.Printf("Starting HTTP listen on %v", gwLis.Addr())

		go func() { listen("http", httpServer.Serve(gwLis)) }()
	}

	if *gatewayPort != 0 {
		loopback, err := client.Dial(context.Background(), fmt.Sprintf("127.0.0.1:%d", *port), grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return fmt.Errorf("grpc-gateway loopback connection failed: %w", err)
		}

		gatewayMux := runtime.NewServeMux()
		for _, f := range s.gatewayRegistrations {
			if err := f(ctx, gatewayMux, loopback); err != nil {
				return fmt.Errorf("grpc-gateway registration failed: %w", err)
			}
		}

		httpServer := &http.Server{Handler: gatewayMux}

		gwLis, err := net.Listen("tcp", fmt.Sprintf("%s:%d", *listenHostname, *gatewayPort))
		if err != nil {
			return err
		}

		core.Log.Printf("Starting gRPC gateway listen on %v", gwLis.Addr())

		go func() { listen("grpc-gateway", httpServer.Serve(gwLis)) }()
	}

	return m.Serve()
}

func interceptorsAsOpts() []grpc.ServerOption {
	unary, streaming := interceptors.ServerInterceptors()

	return []grpc.ServerOption{
		grpc.StreamInterceptor(grpc_middleware.ChainStreamServer(streaming...)),
		grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(unary...)),
	}
}

func listen(what string, err error) {
	if err != nil {
		core.Log.Fatalf("%s: serving failed: %v", what, err)
	}
}
