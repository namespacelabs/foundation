// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"

	"github.com/aws/aws-sdk-go-v2/config"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"namespacelabs.dev/foundation/universe/aws/s3"
)

var (
	awsCredentialsFile = flag.String("aws_credentials_file", "", "Path to the AWS credentials file.")
)

func main() {
	flag.Parse()
	if *awsCredentialsFile == "" {
		log.Fatalf("aws_credentials_file must be set")
	}

	ctx := context.Background()
	// BucketConfigs are passed as additional arguments without flag names.
	for _, jsonBucketConfig := range flag.Args() {
		bc := &s3.BucketConfig{}
		if err := json.Unmarshal([]byte(jsonBucketConfig), bc); err != nil {
			log.Fatalf("Failed to unmarshal bucket config with error: %s", err)
		}

		awsCfg, err := config.LoadDefaultConfig(ctx,
			config.WithRegion(bc.Region),
			config.WithSharedCredentialsFiles([]string{*awsCredentialsFile}))
		if err != nil {
			log.Fatalf("Failed to create s3 client with: %s", err)
		}
		s3client := awss3.NewFromConfig(awsCfg)
		if err = s3.EnsureBucketExists(ctx, s3client, bc); err != nil {
			log.Fatalf("Failed to create s3 bucket with: %s", err)
		}
	}
}
