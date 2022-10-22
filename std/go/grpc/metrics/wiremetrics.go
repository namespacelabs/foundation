// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package metrics

import (
	"context"

	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
)

func Prepare(ctx context.Context, deps ExtensionDeps) error {
	deps.Interceptors.ForClient(grpc_prometheus.UnaryClientInterceptor, grpc_prometheus.StreamClientInterceptor)
	deps.Interceptors.ForServer(grpc_prometheus.UnaryServerInterceptor, grpc_prometheus.StreamServerInterceptor)
	return nil
}
