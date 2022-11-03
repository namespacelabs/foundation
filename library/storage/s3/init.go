// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package s3

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/cenkalti/backoff/v4"
)

const connBackoff = 500 * time.Millisecond

func CreateBucket(ctx context.Context, cfg aws.Config, name string) error {
	cli := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = true
	})

	if name == "" {
		log.Fatal("bucket name is required")
	}

	// Retry until bucket is ready.
	log.Printf("Creating bucket %s.\n", name)
	if err := backoff.Retry(func() error {
		// Speed up bucket creation through faster retries.
		ctx, cancel := context.WithTimeout(ctx, connBackoff)
		defer cancel()

		_, err := cli.CreateBucket(ctx, &s3.CreateBucketInput{
			Bucket: aws.String(name),
		})
		var alreadyExists *types.BucketAlreadyExists
		var alreadyOwned *types.BucketAlreadyOwnedByYou
		if err == nil || errors.As(err, &alreadyExists) || errors.As(err, &alreadyOwned) {
			return nil
		}

		log.Printf("failed to create bucket: %v\n", err)
		return err
	}, backoff.WithContext(backoff.NewConstantBackOff(connBackoff), ctx)); err != nil {
		return err
	}
	log.Printf("Bucket %s created\n", name)

	return nil
}
