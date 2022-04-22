// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package s3

import (
	"context"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type Bucket struct {
	BucketName string
	S3Client   *s3.Client
}

type PutObjectOpt func(*s3.PutObjectInput)

func WithExpiration(expiration time.Time) PutObjectOpt {
	return func(input *s3.PutObjectInput) {
		input.Expires = &expiration
	}
}

func (b Bucket) GetObject(ctx context.Context, key string) (*s3.GetObjectOutput, error) {
	output, err := b.S3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &b.BucketName,
		Key:    &key,
	})
	return output, err
}

func (b Bucket) PutObject(ctx context.Context, key string, body io.Reader, opts ...PutObjectOpt) (*s3.PutObjectOutput, error) {
	input := &s3.PutObjectInput{
		Bucket: &b.BucketName,
		Key:    &key,
		Body:   body,
	}
	for _, opt := range opts {
		opt(input)
	}
	return b.S3Client.PutObject(ctx, input)
}
