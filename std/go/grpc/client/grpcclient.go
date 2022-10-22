// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package client

import (
	"context"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"google.golang.org/grpc"
	"namespacelabs.dev/foundation/std/go/grpc/interceptors"
)

func Dial(ctx context.Context, target string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	unary, streaming := interceptors.ClientInterceptors()

	opts = append(opts,
		grpc.WithStreamInterceptor(grpc_middleware.ChainStreamClient(streaming...)),
		grpc.WithUnaryInterceptor(grpc_middleware.ChainUnaryClient(unary...)),
	)

	return grpc.DialContext(ctx, target, opts...)
}
