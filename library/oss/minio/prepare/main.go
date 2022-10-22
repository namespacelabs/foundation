// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/cenkalti/backoff/v4"
	"namespacelabs.dev/foundation/framework/resources"
	"namespacelabs.dev/foundation/framework/resources/provider"
	s3class "namespacelabs.dev/foundation/library/storage/s3"
)

const (
	providerPkg = "namespacelabs.dev/foundation/library/oss/minio"
	connBackoff = 500 * time.Millisecond
)

func main() {
	intent := &s3class.BucketIntent{}
	ctx, resources := provider.MustPrepare(intent)

	instance, err := prepareInstance(resources, intent)
	if err != nil {
		log.Fatalf("failed to create instance: %v", err)
	}

	resolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		return aws.Endpoint{PartitionID: "aws", URL: instance.Url, SigningRegion: region}, nil
	})

	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(instance.Region),
		config.WithEndpointResolverWithOptions(resolver),
		config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(instance.AccessKey, instance.SecretAccessKey, "" /* session */)))
	if err != nil {
		log.Fatalf("failed to load aws config: %v", err)
	}

	cli := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = true
	})

	if err := createBucket(ctx, cli, instance.BucketName); err != nil {
		log.Fatalf("failed to create bucket: %v", err)
	}

	provider.EmitResult(instance)
}

func prepareInstance(r *resources.Parsed, intent *s3class.BucketIntent) (*s3class.BucketInstance, error) {
	endpoint, err := resources.LookupServerEndpoint(r, fmt.Sprintf("%s:minioServer", providerPkg), "api")
	if err != nil {
		return nil, err
	}

	accessKeyID, err := resources.ReadSecret(r, fmt.Sprintf("%s:minioUser", providerPkg))
	if err != nil {
		return nil, err
	}

	secretAccessKey, err := resources.ReadSecret(r, fmt.Sprintf("%s:minioPassword", providerPkg))
	if err != nil {
		return nil, err
	}

	return &s3class.BucketInstance{
		Region:          intent.Region,
		AccessKey:       string(accessKeyID),
		SecretAccessKey: string(secretAccessKey),
		BucketName:      intent.BucketName,
		Url:             fmt.Sprintf("http://%s", endpoint),
	}, nil
}

func createBucket(ctx context.Context, cli *s3.Client, bucketName string) error {
	// Retry until bucket is ready.
	log.Printf("Creating bucket %s.\n", bucketName)
	if err := backoff.Retry(func() error {
		// Speed up bucket creation through faster retries.
		ctx, cancel := context.WithTimeout(ctx, connBackoff)
		defer cancel()

		_, err := cli.CreateBucket(ctx, &s3.CreateBucketInput{
			Bucket: aws.String(bucketName),
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
	log.Printf("Bucket %s created\n", bucketName)

	return nil
}
