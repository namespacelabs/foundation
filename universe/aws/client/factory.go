// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package client

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"go.opentelemetry.io/contrib/instrumentation/github.com/aws/aws-sdk-go-v2/otelaws"
	"namespacelabs.dev/foundation/std/go/core"
	"namespacelabs.dev/foundation/std/monitoring/tracing"
)

type ClientFactory struct {
	openTelemetry tracing.DeferredTracerProvider
}

func (cf ClientFactory) NewWithCreds(ctx context.Context, optFns ...func(*config.LoadOptions) error) (aws.Config, error) {
	tokenFile := os.Getenv("AWS_WEB_IDENTITY_TOKEN_FILE")
	if tokenFile == "" {
		return aws.Config{}, errors.New("AWS_WEB_IDENTITY_TOKEN_FILE is not set")
	}
	core.Log.Printf("[aws/client] using web identity credentials at %q", tokenFile)

	return cf.New(ctx, optFns...)
}

func (cf ClientFactory) New(ctx context.Context, optFns ...func(*config.LoadOptions) error) (aws.Config, error) {
	cfg, err := config.LoadDefaultConfig(ctx, optFns...)
	if err != nil {
		return aws.Config{}, fmt.Errorf("failed to create aws client config: %w", err)
	}

	if cf.openTelemetry != nil {
		provider, err := cf.openTelemetry.GetTracerProvider()
		if err != nil {
			return aws.Config{}, err
		}

		if provider != nil {
			otelaws.AppendMiddlewares(&cfg.APIOptions, otelaws.WithTracerProvider(provider))
		}
	}

	return cfg, nil
}

func ProvideClientFactory(_ context.Context, _ *ClientFactoryArgs, deps ExtensionDeps) (ClientFactory, error) {
	cf := ClientFactory{openTelemetry: deps.OpenTelemetry}
	return cf, nil
}
