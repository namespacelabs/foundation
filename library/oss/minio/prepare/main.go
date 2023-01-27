// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package main

import (
	"fmt"
	"log"

	"namespacelabs.dev/foundation/framework/resources/provider"
	"namespacelabs.dev/foundation/library/oss/minio"
	"namespacelabs.dev/foundation/library/storage/s3"
)

const providerPkg = "namespacelabs.dev/foundation/library/oss/minio"

func main() {
	ctx, p := provider.MustPrepare[*minio.BucketIntent]()

	endpoint, err := minio.PrepareEndpoint(p.Resources, fmt.Sprintf("%s:server", providerPkg), "api",
		minio.FromSecret(p.Resources, fmt.Sprintf("%s:user", providerPkg)),
		minio.FromSecret(p.Resources, fmt.Sprintf("%s:password", providerPkg)))
	if err != nil {
		log.Fatalf("failed to prepare endpoint: %v", err)
	}

	bucket := &s3.BucketInstance{
		AccessKey:          endpoint.AccessKey,
		SecretAccessKey:    endpoint.SecretAccessKey,
		BucketName:         p.Intent.BucketName,
		Url:                endpoint.PrivateEndpointUrl, // XXX remove.
		PrivateEndpointUrl: endpoint.PrivateEndpointUrl,
	}

	if endpoint.PublicEndpointUrl != "" {
		bucket.PublicUrl = endpoint.PublicEndpointUrl + "/" + p.Intent.BucketName
	}

	if err := minio.EnsureBucket(ctx, bucket); err != nil {
		log.Fatal(err)
	}

	p.EmitResult(bucket)
}
