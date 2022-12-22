// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"namespacelabs.dev/foundation/internal/executor"
	"namespacelabs.dev/foundation/universe/aws/client"
	"namespacelabs.dev/foundation/universe/aws/s3"
	minio "namespacelabs.dev/foundation/universe/storage/minio/creds"
	fns3 "namespacelabs.dev/foundation/universe/storage/s3"
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

	minioCreds, err := getMinioCreds()
	if err != nil {
		return fmt.Errorf("failed to read Minio credentials: %w", err)
	}

	eg := executor.New(ctx, "s3.ensure-buckets")
	for _, bucket := range conf.Bucket {
		bucket := bucket // Close bucket.

		eg.Go(func(ctx context.Context) error {
			b, err := fns3.ProvideBucketWithFactory(ctx, bucket, client.ClientFactory{}, minioCreds)
			if err != nil {
				return err
			}

			if err := s3.EnsureBucketExistsByName(ctx, b.S3Client, bucket.BucketName, bucket.Region); err != nil {
				return fmt.Errorf("failed to create bucket: %w", err)
			}

			return nil
		})
	}

	return eg.Wait()
}

func getMinioCreds() (*minio.Creds, error) {
	minioUser, minioUserOk := os.LookupEnv("MINIO_USER")
	minioPassword, minioPasswordOk := os.LookupEnv("MINIO_PASSWORD")
	if minioUserOk && minioPasswordOk {
		log.Printf("Connecting to minio via new secrets.")
		return &minio.Creds{
			User:     minioUser,
			Password: minioPassword,
		}, nil
	}

	log.Printf("Connecting to AWS S3.")
	return nil, nil
}
