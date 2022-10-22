// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package jaeger

import (
	"context"
	"flag"

	"go.opentelemetry.io/otel/exporters/jaeger"
)

var (
	jaegerEndpoint = flag.String("jaeger_collector_endpoint", "", "Where to push jaeger data to.")
)

func Prepare(ctx context.Context, deps ExtensionDeps) error {
	if *jaegerEndpoint == "" {
		return nil
	}

	exp, err := jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(*jaegerEndpoint)))
	if err != nil {
		return err
	}

	return deps.OpenTelemetry.Register(exp)
}
