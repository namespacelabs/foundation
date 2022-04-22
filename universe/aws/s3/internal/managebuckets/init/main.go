// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"

	"namespacelabs.dev/foundation/universe/aws/s3"
)

var (
	awsCredentialsFile = flag.String("aws_credentials_file", "", "Path to the AWS credentials file.")
)

func main() {
	flag.Parse()
	if *awsCredentialsFile == "" {
		log.Fatalf("aws_credentials_file must be set if localstack_endpoint is not set.")
	}

	ctx := context.Background()
	// BucketConfigs are passed as additional arguments without flag names.
	for _, jsonBucketConfig := range flag.Args() {
		bc := &s3.BucketConfig{}
		if err := json.Unmarshal([]byte(jsonBucketConfig), bc); err != nil {
			log.Fatalf("Failed to unmarshal bucket config with error: %s", err)
		}

		s3client, err := s3.CreateExternalS3Client(ctx, s3.AwsConfig{
			Region:          bc.Region,
			CredentialsPath: *awsCredentialsFile,
		})
		if err != nil {
			log.Fatalf("Failed to create s3 client with: %s", err)
		}
		if err = s3.EnsureBucketExists(ctx, s3client, bc); err != nil {
			log.Fatalf("Failed to create s3 bucket with: %s", err)
		}
	}
}
