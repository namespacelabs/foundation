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
	"namespacelabs.dev/foundation/std/monitoring/tracing"
	minio "namespacelabs.dev/foundation/universe/storage/minio/creds"
)

type ClientFactory struct {
	SharedCredentialsPath string
	MinioCreds            *minio.Creds

	openTelemetry *tracing.DeferredTracerProvider // Optional. Not set for e.g. testing env.
}

type credProvider struct {
	minioCreds *minio.Creds
}

var _ aws.CredentialsProvider = credProvider{}

func (cf credProvider) Retrieve(ctx context.Context) (aws.Credentials, error) {
	return aws.Credentials{
		AccessKeyID:     cf.minioCreds.User,
		SecretAccessKey: cf.minioCreds.Password,
	}, nil
}

func (cf ClientFactory) New(ctx context.Context, optFns ...func(*config.LoadOptions) error) (aws.Config, error) {
	if os.Getenv("AWS_WEB_IDENTITY_TOKEN_FILE") == "" {
		if cf.SharedCredentialsPath != "" {
			optFns = append(optFns, config.WithSharedCredentialsFiles([]string{cf.SharedCredentialsPath}))
		} else if cf.MinioCreds != nil {
			optFns = append(optFns, config.WithCredentialsProvider(credProvider{cf.MinioCreds}))
		} else {
			return aws.Config{}, errors.New("when running without universe/aws/irsa, aws credentials are required to be set")
		}
	}

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
	cf := ClientFactory{openTelemetry: &deps.OpenTelemetry, MinioCreds: deps.MinioCreds}
	if deps.Credentials != nil {
		cf.SharedCredentialsPath = deps.Credentials.Path
	}
	return cf, nil
}
