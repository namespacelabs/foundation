// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package postgres

import (
	"context"

	"go.opentelemetry.io/otel/trace"
	"namespacelabs.dev/foundation/framework/resources"
)

type Factory struct {
	res    *resources.Parsed
	tp     trace.TracerProvider
	client string
}

func ProvideFactory(ctx context.Context, args *FactoryArgs, deps ExtensionDeps) (Factory, error) {
	res, err := resources.LoadResources()
	if err != nil {
		return Factory{}, err
	}

	tp, err := deps.OpenTelemetry.GetTracerProvider()
	if err != nil {
		return Factory{}, err
	}

	return Factory{res, tp, args.GetClient()}, nil
}

func (f Factory) Provide(ctx context.Context, ref string) (*DB, error) {
	return ConnectToResource(ctx, f.res, ref, f.tp, f.client, nil)
}
