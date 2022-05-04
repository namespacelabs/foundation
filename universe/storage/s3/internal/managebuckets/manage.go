// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"google.golang.org/protobuf/encoding/protojson"
	"namespacelabs.dev/foundation/internal/executor"
	"namespacelabs.dev/foundation/universe/aws/s3"
	devs3 "namespacelabs.dev/foundation/universe/development/localstack/s3"
	fns3 "namespacelabs.dev/foundation/universe/storage/s3"
)

var (
	awsCredentialsFile = flag.String("aws_credentials_file", "", "Path to the AWS credentials file.")
	configuredBuckets  = flag.String("storage_s3_configured_buckets_protojson", "", "A serialized MultipleBucketArgs with all of the bucket configurations.")
	localstackEndpoint = flag.String("storage_s3_localstack_endpoint", "", "If set, use localstack.")
)

func main() {
	log.SetFlags(log.Lmicroseconds | log.LstdFlags)

	flag.Parse()

	if err := apply(context.Background()); err != nil {
		log.Fatal(err)
	}
}

func apply(ctx context.Context) error {
	conf := &fns3.MultipleBucketArgs{}
	if err := protojson.Unmarshal([]byte(*configuredBuckets), conf); err != nil {
		return err
	}

	ex, wait := executor.New(ctx)

	for _, bucket := range conf.Bucket {
		bucket := bucket // Close bucket.

		ex.Go(func(ctx context.Context) error {
			if bucket.Region == "" {
				return fmt.Errorf("%s: missing region", bucket.BucketName)
			}

			s3client, err := createClient(ctx, bucket.Region)
			if err != nil {
				return fmt.Errorf("failed to create s3 client: %w", err)
			}

			if err = s3.EnsureBucketExistsByName(ctx, s3client, bucket.BucketName, bucket.Region); err != nil {
				return fmt.Errorf("failed to create bucket: %w", err)
			}

			return nil
		})
	}

	return wait()
}

func createClient(ctx context.Context, region string) (*awss3.Client, error) {
	if *localstackEndpoint != "" {
		return devs3.CreateLocalstackS3Client(ctx, devs3.LocalstackConfig{
			Region:             region,
			LocalstackEndpoint: *localstackEndpoint,
		})
	}

	var opts []func(*config.LoadOptions) error
	opts = append(opts, config.WithRegion(region))

	if os.Getenv("AWS_WEB_IDENTITY_TOKEN_FILE") == "" {
		if *awsCredentialsFile == "" {
			return nil, errors.New("when running without universe/aws/irsa, aws credentials are required to be set")
		}

		opts = append(opts, config.WithSharedCredentialsFiles([]string{*awsCredentialsFile}))
	}

	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, err
	}

	return awss3.NewFromConfig(cfg), nil
}
