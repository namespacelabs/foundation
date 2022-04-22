// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package s3

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/cenkalti/backoff/v4"
)

const connBackoff = 500 * time.Millisecond

// EnsureBucketExists creates the requested bucket before it is used.
func EnsureBucketExists(ctx context.Context, s3client *s3.Client, bc *BucketConfig) error {
	log.Printf("Creating bucket %s in region: %s\n", bc.BucketName, bc.Region)
	err := backoff.Retry(func() error {
		log.Printf("Connecting to S3 stack.")
		_, err := s3client.ListBuckets(ctx, &s3.ListBucketsInput{})
		if err != nil {
			log.Printf("Failed to list buckets: %v", err)
		}
		return err
	}, backoff.WithContext(backoff.NewConstantBackOff(connBackoff), ctx))
	if err != nil {
		return fmt.Errorf("failed to connect to S3 stack with error: %w", err)
	}

	_, err = s3client.CreateBucket(ctx, &s3.CreateBucketInput{
		CreateBucketConfiguration: &types.CreateBucketConfiguration{
			LocationConstraint: types.BucketLocationConstraint(bc.Region),
		},
		Bucket: &bc.BucketName,
	})
	if err != nil {
		var e *types.BucketAlreadyOwnedByYou
		if !errors.As(err, &e) {
			return fmt.Errorf("failed to create bucket: %w", err)
		}
	}
	return nil
}
