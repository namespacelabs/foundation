// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package s3

import (
	"context"
	"flag"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"google.golang.org/protobuf/encoding/protojson"
	fns3 "namespacelabs.dev/foundation/universe/aws/s3"
	devs3 "namespacelabs.dev/foundation/universe/development/localstack/s3"
)

var (
	configuredBuckets  = flag.String("storage_s3_configured_buckets_protojson", "", "A serialized MultipleBucketArgs with all of the bucket configurations.")
	localstackEndpoint = flag.String("storage_s3_localstack_endpoint", "", "If set, use localstack.")
)

func ProvideBucket(ctx context.Context, args *BucketArgs, deps ExtensionDeps) (*fns3.Bucket, error) {
	conf := &MultipleBucketArgs{}
	if err := protojson.Unmarshal([]byte(*configuredBuckets), conf); err != nil {
		return nil, err
	}

	for _, bucket := range conf.Bucket {
		if bucket.BucketName == args.BucketName {
			if bucket.Region == "" {
				return nil, fmt.Errorf("%s: bucket is missing a region configuration", args.BucketName)
			}

			s3client, err := createClient(ctx, deps, bucket.Region)
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

func createClient(ctx context.Context, deps ExtensionDeps, region string) (*s3.Client, error) {
	if *localstackEndpoint != "" {
		return devs3.CreateLocalstackS3Client(ctx, devs3.LocalstackConfig{
			Region:             region,
			LocalstackEndpoint: *localstackEndpoint,
		})
	}

	cfg, err := deps.ClientFactory.New(ctx, config.WithRegion(region))
	if err != nil {
		return nil, err
	}

	return s3.NewFromConfig(cfg), nil
}
