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
	devs3 "namespacelabs.dev/foundation/universe/development/localstack/s3"
)

var (
	localstackEndpoint = flag.String("init_localstack_endpoint", "", "Localstack endpoint configuration.")
)

func main() {
	flag.Parse()
	log.Printf("Starting with endpoint %s\n", *localstackEndpoint)

	ctx := context.Background()
	// BucketConfigs are passed as additional arguments without flag names.
	for _, jsonBucketConfig := range flag.Args() {
		bc := &devs3.BucketConfig{}
		if err := json.Unmarshal([]byte(jsonBucketConfig), bc); err != nil {
			log.Fatalf("Failed to unmarshal bucket config with error: %s", err)
		}

		s3client, err := devs3.CreateLocalstackS3Client(ctx, devs3.LocalstackConfig{
			Region:             bc.Region,
			LocalstackEndpoint: *localstackEndpoint,
		})
		if err != nil {
			log.Fatalf("Failed to create s3 client with: %v", err)
		}
		if err := s3.EnsureBucketExists(ctx, s3client, bc); err != nil {
			log.Fatalf("Failed to create s3 bucket: %v", err)
		}
	}
}
