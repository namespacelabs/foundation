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
	"namespacelabs.dev/foundation/framework/resources/provider"
	"namespacelabs.dev/foundation/library/oss/localstack"
	"namespacelabs.dev/foundation/library/storage/s3"
)

const (
	providerPkg = "namespacelabs.dev/foundation/library/oss/localstack"

	// Any non-empty access key is valid for localstack.
	// https://github.com/localstack/localstack/issues/62#issuecomment-294749459
	accessKey       = "localstack"
	secretAccessKey = "localstack"
)

func main() {
	ctx, p := provider.MustPrepare[*s3.BucketIntent]()

	server := &localstack.ClusterInstance{}
	if err := p.Resources.Unmarshal(fmt.Sprintf("%s:cluster", providerPkg), server); err != nil {
		log.Fatalf("unable to read required resource \"cluster\": %v", err)
	}

	instance := &s3.BucketInstance{
		Region:             p.Intent.Region,
		BucketName:         p.Intent.BucketName,
		AccessKey:          accessKey,
		SecretAccessKey:    secretAccessKey,
		Url:                fmt.Sprintf("http://%s", server.Endpoint),
		PrivateEndpointUrl: fmt.Sprintf("http://%s", server.Endpoint),
	}

	if server.PublicBaseUrl != "" {
		instance.PublicUrl = server.PublicBaseUrl + "/" + p.Intent.BucketName
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
