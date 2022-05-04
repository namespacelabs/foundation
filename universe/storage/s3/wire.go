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
)

var (
	configuredBuckets = flag.String("storage_s3_configured_buckets_protojson", "", "A serialized MultipleBucketArgs with all of the bucket configurations.")
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

			cfg, err := deps.ClientFactory.New(ctx, config.WithRegion(bucket.Region))
			if err != nil {
				return nil, err
			}

			s3client := s3.NewFromConfig(cfg)

			return &fns3.Bucket{
				BucketName: args.BucketName,
				S3Client:   s3client,
			}, nil
		}
	}

	return nil, fmt.Errorf("%s: no bucket configuration available", args.BucketName)
}
