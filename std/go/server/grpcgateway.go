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

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"namespacelabs.dev/foundation/std/go/core"
	"namespacelabs.dev/foundation/std/go/grpc/client"
)

var gatewayPort = flag.Int("gateway_port", 0, "Port to listen gRPC Gateway on.")

func setupGrpcGateway(ctx context.Context, gatewayRegistrations []func(context.Context, *runtime.ServeMux, *grpc.ClientConn) error) error {
	if *gatewayPort == 0 || len(gatewayRegistrations) == 0 {
		return nil
	}

	// context.Background's use is deliberate here; don't want the client connections to be canceled after initialization.
	loopback, err := client.Dial(context.Background(), fmt.Sprintf("127.0.0.1:%d", *port), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("grpc-gateway loopback connection failed: %w", err)
	}

	gatewayMux := runtime.NewServeMux()
	for _, f := range gatewayRegistrations {
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

	go func() { checkReturn("grpc-gateway", httpServer.Serve(gwLis)) }()

	return nil
}
