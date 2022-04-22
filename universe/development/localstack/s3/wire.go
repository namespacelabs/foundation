// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package s3

import (
	"context"
	"flag"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"namespacelabs.dev/foundation/std/go/core"
	fns3 "namespacelabs.dev/foundation/universe/aws/s3"
)

var (
	localstackEndpoint = flag.String("localstack_endpoint", "", "Localstack endpoint configuration.")
)

type LocalstackConfig struct {
	Region, LocalstackEndpoint string
}

func createLocalStackConfig(ctx context.Context, c LocalstackConfig) (aws.Config, error) {
	customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		return aws.Endpoint{
			PartitionID:   "aws",
			URL:           c.LocalstackEndpoint,
			SigningRegion: region,
		}, nil
	})
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(c.Region),
		// Specify a custom resolver to be able to point to localstack's endpoint.
		config.WithEndpointResolverWithOptions(customResolver))
	if err != nil {
		return aws.Config{}, fmt.Errorf("failed to load AWS config with error: %w, for endpoint %s", err, *localstackEndpoint)
	}
	return cfg, nil
}

func CreateLocalstackS3Client(ctx context.Context, config LocalstackConfig) (*s3.Client, error) {
	cfg, err := createLocalStackConfig(ctx, config)
	if err != nil {
		return nil, err
	}
	s3client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		// Make sure the bucket is encoded into the URL after domain is resolved, not as a subdomain.
		// TODO UsePathStyle is deprecated - use it only if localstack is used before we can dynamically add DNS entries.
		o.UsePathStyle = true
	})
	return s3client, nil
}

func ProvideBucket(ctx context.Context, config *BucketConfig, deps ExtensionDeps) (*fns3.Bucket, error) {
	s3client, err := CreateLocalstackS3Client(ctx,
		LocalstackConfig{
			Region:             config.Region,
			LocalstackEndpoint: *localstackEndpoint,
		})
	if err != nil {
		return nil, err
	}

	// Asynchronously wait until a database connection is ready.
	deps.ReadinessCheck.RegisterFunc(
		fmt.Sprintf("localstack readiness: %s", core.InstantiationPathFromContext(ctx)),
		func(ctx context.Context) error {
			_, err := s3client.ListBuckets(ctx, &s3.ListBucketsInput{})
			return err
		})

	return &fns3.Bucket{
		BucketName: config.BucketName,
		S3Client:   s3client,
	}, nil
}
