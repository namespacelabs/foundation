// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package metrics

import (
	"context"

	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
)

func Prepare(ctx context.Context, deps *ExtensionDeps) error {
	deps.Interceptors.Add(grpc_prometheus.UnaryServerInterceptor, grpc_prometheus.StreamServerInterceptor)
	return nil
}
