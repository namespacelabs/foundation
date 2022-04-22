// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package s3

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"namespacelabs.dev/foundation/std/go/core"
)

type AwsConfig struct {
	Region, CredentialsPath string
}

func createExternalConfig(ctx context.Context, c AwsConfig) (aws.Config, error) {
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithSharedCredentialsFiles(
			[]string{c.CredentialsPath},
		),
		config.WithRegion(c.Region))
	if err != nil {
		return aws.Config{}, fmt.Errorf("Failed to load AWS config with error: %v, for config %#v", err, c)
	}
	return cfg, nil
}

func CreateExternalS3Client(ctx context.Context, c AwsConfig) (*s3.Client, error) {
	cfg, err := createExternalConfig(ctx, c)
	if err != nil {
		return nil, err
	}
	s3client := s3.NewFromConfig(cfg)
	return s3client, nil
}

func ProvideBucket(ctx context.Context, config *BucketConfig, deps ExtensionDeps) (*Bucket, error) {
	s3client, err := CreateExternalS3Client(ctx, AwsConfig{
		Region:          config.Region,
		CredentialsPath: deps.Credentials.Path})
	if err != nil {
		return nil, err
	}

	// Asynchronously wait until a database connection is ready.
	deps.ReadinessCheck.RegisterFunc(
		fmt.Sprintf("readiness:%s", core.InstantiationPathFromContext(ctx)),
		func(ctx context.Context) error {
			_, err := s3client.ListBuckets(ctx, &s3.ListBucketsInput{})
			return err
		})

	return &Bucket{
		BucketName: config.BucketName,
		S3Client:   s3client,
	}, nil
}
