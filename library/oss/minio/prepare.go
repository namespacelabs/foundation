// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package minio

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"namespacelabs.dev/foundation/framework/resources"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/library/storage/s3"
)

func EnsureBucket(ctx context.Context, instance *s3.BucketInstance) error {
	resolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		return aws.Endpoint{PartitionID: "aws", URL: instance.Url, SigningRegion: region}, nil
	})

	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(instance.Region),
		config.WithEndpointResolverWithOptions(resolver),
		config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(instance.AccessKey, instance.SecretAccessKey, "" /* session */)))
	if err != nil {
		return fnerrors.New("failed to load aws config: %v", err)
	}

	if err := s3.CreateBucket(ctx, cfg, instance.BucketName); err != nil {
		return fnerrors.New("failed to create bucket: %w", err)
	}

	return nil
}

func FromStatic(r *resources.Parsed, secretValue []byte) func() ([]byte, error) {
	return func() ([]byte, error) { return secretValue, nil }
}

func FromSecret(r *resources.Parsed, secretName string) func() ([]byte, error) {
	return func() ([]byte, error) {
		return resources.ReadSecret(r, secretName)
	}
}

type deferredFunc func() ([]byte, error)

type Endpoint struct {
	AccessKey          string
	SecretAccessKey    string
	PrivateEndpoint    string
	PrivateEndpointUrl string
	PublicEndpointUrl  string
}

func PrepareEndpoint(r *resources.Parsed, serverRef, serviceName string, accessKey, secretKey deferredFunc) (*Endpoint, error) {
	x, err := resources.LookupServerEndpoint(r, serverRef, serviceName)
	if err != nil {
		return nil, err
	}

	accessKeyID, err := accessKey()
	if err != nil {
		return nil, err
	}

	secretAccessKey, err := secretKey()
	if err != nil {
		return nil, err
	}

	ingress, err := resources.LookupServerFirstIngress(r, serverRef, serviceName)
	if err != nil {
		return nil, err
	}

	endpoint := &Endpoint{
		AccessKey:          string(accessKeyID),
		SecretAccessKey:    string(secretAccessKey),
		PrivateEndpoint:    x,
		PrivateEndpointUrl: fmt.Sprintf("http://%s", x),
	}

	if ingress != nil {
		endpoint.PublicEndpointUrl = *ingress
	}

	return endpoint, nil
}
