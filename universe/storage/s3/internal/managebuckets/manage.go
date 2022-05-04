// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"namespacelabs.dev/foundation/internal/executor"
	"namespacelabs.dev/foundation/universe/aws/client"
	"namespacelabs.dev/foundation/universe/aws/s3"
	fns3 "namespacelabs.dev/foundation/universe/storage/s3"
)

var (
	awsCredentialsFile = flag.String("aws_credentials_file", "", "Path to the AWS credentials file.")
)

func main() {
	log.SetFlags(log.Lmicroseconds | log.LstdFlags)

	flag.Parse()

	if err := apply(context.Background()); err != nil {
		log.Fatal(err)
	}
}

func apply(ctx context.Context) error {
	conf, err := fns3.ProvidedConfiguration()
	if err != nil {
		return fmt.Errorf("failed to unmarshal: %w", err)
	}

	ex, wait := executor.New(ctx)

	for _, bucket := range conf.Bucket {
		bucket := bucket // Close bucket.

		ex.Go(func(ctx context.Context) error {
			if bucket.Region == "" {
				return fmt.Errorf("%s: missing region", bucket.BucketName)
			}

			b, err := fns3.ProvideBucketWithFactory(ctx, bucket, client.ClientFactory{
				SharedCredentialsPath: *awsCredentialsFile,
			})
			if err != nil {
				return err
			}

			if err = s3.EnsureBucketExistsByName(ctx, b.S3Client, bucket.BucketName, bucket.Region); err != nil {
				return fmt.Errorf("failed to create bucket: %w", err)
			}

			return nil
		})
	}

	return wait()
}
