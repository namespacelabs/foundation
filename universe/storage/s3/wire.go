// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package s3

import (
	"context"
	"flag"
	"fmt"
	sync "sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"google.golang.org/protobuf/encoding/protojson"
	"namespacelabs.dev/foundation/std/go/core"
	"namespacelabs.dev/foundation/universe/aws/client"
	fns3 "namespacelabs.dev/foundation/universe/aws/s3"
	devs3 "namespacelabs.dev/foundation/universe/development/localstack/s3"
	minio "namespacelabs.dev/foundation/universe/storage/minio/creds"
)

var (
	configuredBuckets  = flag.String("storage_s3_configured_buckets_protojson", "", "A serialized MultipleBucketArgs with all of the bucket configurations.")
	localstackEndpoint = flag.String("storage_s3_localstack_endpoint", "", "If set, use localstack.")
	minioEndpoint      = flag.String("storage_s3_minio_endpoint", "", "If set, use minio.")

	parsedConfiguration *MultipleBucketArgs
	parseOnce           sync.Once
	parseErr            error
)

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

func ProvidedConfiguration() (*MultipleBucketArgs, error) {
	parseOnce.Do(func() {
		parsedConfiguration = &MultipleBucketArgs{}
		parseErr = protojson.Unmarshal([]byte(*configuredBuckets), parsedConfiguration)
	})
	return parsedConfiguration, parseErr
}

func ProvideBucket(ctx context.Context, args *BucketArgs, deps ExtensionDeps) (*fns3.Bucket, error) {
	return ProvideBucketWithFactory(ctx, args, deps.ClientFactory, deps.MinioCreds)
}

func ProvideBucketWithFactory(ctx context.Context, args *BucketArgs, factory client.ClientFactory, minioCreds *minio.Creds) (*fns3.Bucket, error) {
	conf, err := ProvidedConfiguration()
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal: %w", err)
	}

	for _, bucket := range conf.Bucket {
		if bucket.BucketName == args.BucketName {
			if bucket.Region == "" {
				return nil, fmt.Errorf("%s: bucket is missing a region configuration", args.BucketName)
			}

			s3client, err := createClient(ctx, factory, minioCreds, bucket.Region)
			if err != nil {
				return nil, err
			}

			return &fns3.Bucket{
				BucketName: args.BucketName,
				S3Client:   s3client,
			}, nil
		}
	}

	return nil, fmt.Errorf("%s: no bucket configuration available", args.BucketName)
}

func createClient(ctx context.Context, factory client.ClientFactory, minioCreds *minio.Creds, region string) (*s3.Client, error) {
	// TODO move client creation with a dedicated endpoint into a factory.
	if *localstackEndpoint != "" {
		return devs3.CreateLocalstackS3Client(ctx, devs3.LocalstackConfig{
			Region:             region,
			LocalstackEndpoint: *localstackEndpoint,
		})
	}

	var loadOptFns [](func(*config.LoadOptions) error)
	var optFns [](func(*s3.Options))

	loadOptFns = append(loadOptFns, config.WithRegion(region))

	var cfg aws.Config
	var err error
	if *minioEndpoint != "" {
		core.Log.Printf("[storage/s3] creating minio client: %s", *minioEndpoint)

		resolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{
				PartitionID:   "aws",
				URL:           *minioEndpoint,
				SigningRegion: region,
			}, nil
		})

		loadOptFns = append(loadOptFns,
			config.WithEndpointResolverWithOptions(resolver),
			config.WithCredentialsProvider(credProvider{minioCreds: minioCreds}))

		optFns = append(optFns, func(o *s3.Options) {
			o.UsePathStyle = true
		})

		cfg, err = factory.New(ctx, loadOptFns...)
	} else {
		cfg, err = factory.NewWithCreds(ctx, loadOptFns...)
	}

	if err != nil {
		return nil, err
	}

	// Verify we can retrieve credentials. Had a weird failure previously, where
	// the lack of credentials still led to requests being signed, but without
	// access key.
	if _, err := cfg.Credentials.Retrieve(ctx); err != nil {
		return nil, fmt.Errorf("failed to retrieve credentials: %w", err)
	}

	return s3.NewFromConfig(cfg, optFns...), nil
}
