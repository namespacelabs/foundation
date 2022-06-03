// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

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
	SharedCredentialsPath string

	openTelemetry tracing.DeferredTracerProvider
}

func (cf ClientFactory) NewWithCreds(ctx context.Context, optFns ...func(*config.LoadOptions) error) (aws.Config, error) {
	if tokenFile := os.Getenv("AWS_WEB_IDENTITY_TOKEN_FILE"); tokenFile == "" {
		if cf.SharedCredentialsPath == "" {
			return aws.Config{}, errors.New("when running without universe/aws/irsa, aws credentials are required to be set")
		}

		core.Log.Printf("[aws/client] using shared credentials at %q", cf.SharedCredentialsPath)

		optFns = append(optFns, config.WithSharedCredentialsFiles([]string{cf.SharedCredentialsPath}))
	} else {
		core.Log.Printf("[aws/client] using web identity credentials at %q", tokenFile)
	}

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
	if deps.Credentials != nil {
		cf.SharedCredentialsPath = deps.Credentials.Path
	}
	return cf, nil
}
