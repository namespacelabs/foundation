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
func EnsureBucketExists(ctx context.Context, client *s3.Client, bc *BucketConfig) error {
	return EnsureBucketExistsByName(ctx, client, bc.BucketName, bc.Region)
}

func EnsureBucketExistsByName(ctx context.Context, client *s3.Client, name, region string) error {
	log.Printf("%s (%s): creating bucket...\n", name, region)
	if err := backoff.Retry(func() error {
		input := &s3.CreateBucketInput{
			Bucket: &name,
		}

		if region != "" {
			input.CreateBucketConfiguration = &types.CreateBucketConfiguration{
				LocationConstraint: types.BucketLocationConstraint(region),
			}
		}

		if _, err := client.CreateBucket(ctx, input); err != nil {
			var alreadyExists *types.BucketAlreadyExists
			var alreadyOwned *types.BucketAlreadyOwnedByYou
			if errors.As(err, &alreadyExists) || errors.As(err, &alreadyOwned) {
				log.Printf("%s (%s): bucket already exists.\n", name, region)
				return nil
			}

			err = fmt.Errorf("failed to create bucket: %w", err)
			log.Println(err)
			return err
		}

		log.Printf("%s (%s): bucket created.\n", name, region)

		return nil
	}, backoff.WithContext(backoff.NewConstantBackOff(connBackoff), ctx)); err != nil {
		return fmt.Errorf("failed to create S3 bucket: %w", err)
	}

	return nil
}
