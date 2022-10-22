// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package main

import (
	"context"
	"fmt"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"namespacelabs.dev/foundation/internal/fnerrors"

	clusterservice "github.com/envoyproxy/go-control-plane/envoy/service/cluster/v3"
	discoverygrpc "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	endpointservice "github.com/envoyproxy/go-control-plane/envoy/service/endpoint/v3"
	listenerservice "github.com/envoyproxy/go-control-plane/envoy/service/listener/v3"
	routeservice "github.com/envoyproxy/go-control-plane/envoy/service/route/v3"
	runtimeservice "github.com/envoyproxy/go-control-plane/envoy/service/runtime/v3"
	secretservice "github.com/envoyproxy/go-control-plane/envoy/service/secret/v3"
	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/envoyproxy/go-control-plane/pkg/server/v3"
	"github.com/envoyproxy/go-control-plane/pkg/test/v3"
)

const (
	grpcKeepaliveTime        = 30 * time.Second
	grpcKeepaliveTimeout     = 5 * time.Second
	grpcKeepaliveMinTime     = 30 * time.Second
	grpcMaxConcurrentStreams = 1000000
)

type XdsServer struct {
	grpcServer *grpc.Server
	xdsServer  server.Server
}

func NewXdsServer(ctx context.Context, snapshotCache cache.SnapshotCache, debug bool) *XdsServer {
	// gRPC golang library sets a very small upper bound for the number gRPC/h2
	// streams over a single TCP connection. If a proxy multiplexes requests over
	// a single connection to the management server, then it might lead to
	// availability problems. Keepalive timeouts based on connection_keepalive parameter https://www.envoyproxy.io/docs/envoy/latest/configuration/overview/examples#dynamic
	var grpcOptions []grpc.ServerOption
	grpcOptions = append(grpcOptions,
		grpc.MaxConcurrentStreams(grpcMaxConcurrentStreams),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			Time:    grpcKeepaliveTime,
			Timeout: grpcKeepaliveTimeout,
		}),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             grpcKeepaliveMinTime,
			PermitWithoutStream: true,
		}),
	)

	cb := &test.Callbacks{Debug: debug}

	return &XdsServer{
		grpcServer: grpc.NewServer(grpcOptions...),
		xdsServer:  server.NewServer(ctx, snapshotCache, cb),
	}
}

func (x *XdsServer) RegisterServices() {
	discoverygrpc.RegisterAggregatedDiscoveryServiceServer(x.grpcServer, x.xdsServer)
	endpointservice.RegisterEndpointDiscoveryServiceServer(x.grpcServer, x.xdsServer)
	clusterservice.RegisterClusterDiscoveryServiceServer(x.grpcServer, x.xdsServer)
	routeservice.RegisterRouteDiscoveryServiceServer(x.grpcServer, x.xdsServer)
	listenerservice.RegisterListenerDiscoveryServiceServer(x.grpcServer, x.xdsServer)
	secretservice.RegisterSecretDiscoveryServiceServer(x.grpcServer, x.xdsServer)
	runtimeservice.RegisterRuntimeDiscoveryServiceServer(x.grpcServer, x.xdsServer)
}

// Serve serves the GRPC endpoint on the given port, returning only when it fails or is stopped.
func (x *XdsServer) Serve(port uint32) error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fnerrors.InternalError("xDS server failed to listen on %d: %w", port, err)
	}

	if err = x.grpcServer.Serve(lis); err != nil {
		return fnerrors.InternalError("failed to run the xDS server control loop on %d: %w", port, err)
	}

	return nil
}

// Stop requests a graceful stop of the xDS GRPC server.
func (x *XdsServer) Stop() {
	x.grpcServer.GracefulStop()
}

// Start runs the xDRS GRPC server on the given port, returning
// only when the server is stopped by the context closing, or when
// it fails.
func (x *XdsServer) Start(ctx context.Context, port uint32) error {
	errChan := make(chan error)

	go func() {
		errChan <- x.Serve(port)
	}()

	select {
	case <-ctx.Done():
		<-errChan // Wait for Serve to return.
		x.Stop()
		return ctx.Err()
	case err := <-errChan:
		return err
	}
}
