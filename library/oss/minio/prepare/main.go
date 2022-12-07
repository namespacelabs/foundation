// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package main

import (
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"namespacelabs.dev/foundation/framework/resources"
	"namespacelabs.dev/foundation/framework/resources/provider"
	"namespacelabs.dev/foundation/library/storage/s3"
)

const providerPkg = "namespacelabs.dev/foundation/library/oss/minio"

func main() {
	ctx, p := provider.MustPrepare[*s3.BucketIntent]()

	instance, err := prepareInstance(p.Resources, p.Intent)
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

	if err := s3.CreateBucket(ctx, cfg, instance.BucketName); err != nil {
		log.Fatalf("failed to create bucket: %v", err)
	}

	p.EmitResult(instance)
}

func prepareInstance(r *resources.Parsed, intent *s3.BucketIntent) (*s3.BucketInstance, error) {
	endpoint, err := resources.LookupServerEndpoint(r, fmt.Sprintf("%s:server", providerPkg), "api")
	if err != nil {
		return nil, err
	}

	accessKeyID, err := resources.ReadSecret(r, fmt.Sprintf("%s:user", providerPkg))
	if err != nil {
		return nil, err
	}

	secretAccessKey, err := resources.ReadSecret(r, fmt.Sprintf("%s:password", providerPkg))
	if err != nil {
		return nil, err
	}

	return &s3.BucketInstance{
		Region:          intent.Region,
		AccessKey:       string(accessKeyID),
		SecretAccessKey: string(secretAccessKey),
		BucketName:      intent.BucketName,
		Url:             fmt.Sprintf("http://%s", endpoint),
	}, nil
}
