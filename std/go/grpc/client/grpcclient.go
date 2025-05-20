// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package client

import (
	"os"
	"time"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"namespacelabs.dev/foundation/std/go/grpc/interceptors"
)

func NewClient(target string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	unary, streaming := interceptors.ClientInterceptors()

	opts = append(opts,
		grpc.WithStreamInterceptor(grpc_middleware.ChainStreamClient(streaming...)),
		grpc.WithUnaryInterceptor(grpc_middleware.ChainUnaryClient(unary...)),
	)

	if svccfg := os.Getenv("FOUNDATION_GRPC_DEFAULT_SERVICE_CONFIG"); svccfg != "" {
		opts = append(opts, grpc.WithDefaultServiceConfig(svccfg))
	}

	opts = append(opts, grpc.WithConnectParams(grpc.ConnectParams{
		Backoff: backoff.Config{
			BaseDelay:  500 * time.Millisecond,
			Multiplier: 1.6,
			MaxDelay:   10 * time.Second,
		}}))

	return grpc.NewClient(target, opts...)
}
