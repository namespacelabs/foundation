// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package global

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/codes"
	"namespacelabs.dev/foundation/framework/rpcerrors"
	"namespacelabs.dev/foundation/std/go/core"
)

func ProvideTracer(ctx context.Context, _ *NoArgs, deps ExtensionDeps) (trace.Tracer, error) {
	t, err := deps.TracerProvider.GetTracerProvider()
	if err != nil {
		return nil, err
	}

	p := core.InstantiationPathFromContext(ctx)
	if p == nil || p.Last() == "" {
		return nil, rpcerrors.Errorf(codes.Internal, "expected instantiation path")
	}

	return t.Tracer(p.Last().GetPackageName(), trace.WithInstrumentationAttributes(attribute.String("ns.package_name", p.String()))), nil
}

func ProvideMeter(ctx context.Context, _ *NoArgs, deps ExtensionDeps) (metric.Meter, error) {
	p := core.InstantiationPathFromContext(ctx)
	if p == nil || p.Last() == "" {
		return nil, rpcerrors.Errorf(codes.Internal, "expected instantiation path")
	}

	return deps.MeterProvider.Meter(p.Last().GetPackageName()), nil
}
