// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package client

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"go.opentelemetry.io/contrib/instrumentation/github.com/aws/aws-sdk-go-v2/otelaws"
	"namespacelabs.dev/foundation/std/monitoring/tracing"
)

type ClientFactory struct {
	credsPath     string
	openTelemetry tracing.DeferredTracerProvider
}

func (cf ClientFactory) New(ctx context.Context, optFns ...func(*config.LoadOptions) error) (aws.Config, error) {
	optFns = append(optFns, config.WithSharedCredentialsFiles([]string{cf.credsPath}))

	cfg, err := config.LoadDefaultConfig(ctx, optFns...)
	if err != nil {
		return aws.Config{}, fmt.Errorf("failed to create aws client config: %w", err)
	}

	provider, err := cf.openTelemetry.GetTracerProvider()
	if err != nil {
		return aws.Config{}, err
	}

	if provider != nil {
		otelaws.AppendMiddlewares(&cfg.APIOptions, otelaws.WithTracerProvider(provider))
	}

	return cfg, nil
}

func ProvideClientFactory(_ context.Context, _ *ClientFactoryArgs, deps ExtensionDeps) (ClientFactory, error) {
	return ClientFactory{credsPath: deps.Credentials.Path, openTelemetry: deps.OpenTelemetry}, nil
}
