// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package k8sdetector

import (
	"context"
	"os"

	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
)

func Prepare(ctx context.Context, deps ExtensionDeps) error {
	deps.Detector.Register(detector{})
	return nil
}

type detector struct{}

func (detector) Detect(ctx context.Context) (*resource.Resource, error) {
	return resource.NewWithAttributes(semconv.SchemaURL,
		semconv.K8SNamespaceName(os.Getenv("TRACING_K8S_NAMESPACE")),
		semconv.K8SPodName(os.Getenv("TRACING_K8S_POD_NAME")),
		semconv.K8SPodUID(os.Getenv("TRACING_K8S_POD_UID")),
		semconv.K8SNodeName(os.Getenv("TRACING_K8S_NODE_NAME")),
	), nil
}
