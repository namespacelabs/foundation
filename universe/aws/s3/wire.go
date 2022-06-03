// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package s3

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"namespacelabs.dev/foundation/std/go/core"
)

type AwsConfig struct {
	Region, CredentialsPath string
}

func ProvideBucket(ctx context.Context, bc *BucketConfig, deps ExtensionDeps) (*Bucket, error) {
	cfg, err := deps.ClientFactory.NewWithCreds(ctx, config.WithRegion(bc.Region))
	if err != nil {
		return nil, err
	}
	s3client := s3.NewFromConfig(cfg)

	// Asynchronously wait until the bucket is available.
	deps.ReadinessCheck.RegisterFunc(
		fmt.Sprintf("readiness:%s", core.InstantiationPathFromContext(ctx)),
		func(ctx context.Context) error {
			_, err := s3client.ListBuckets(ctx, &s3.ListBucketsInput{})
			return err
		})

	return &Bucket{
		BucketName: bc.BucketName,
		S3Client:   s3client,
	}, nil
}
