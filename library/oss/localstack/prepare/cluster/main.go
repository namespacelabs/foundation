// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package main

import (
	"fmt"
	"log"

	"namespacelabs.dev/foundation/framework/resources"
	"namespacelabs.dev/foundation/framework/resources/provider"
	"namespacelabs.dev/foundation/library/oss/localstack"
)

const providerPkg = "namespacelabs.dev/foundation/library/oss/localstack"

func main() {
	_, p := provider.MustPrepare[*localstack.ClusterIntent]()

	endpoint, err := resources.LookupServerEndpoint(p.Resources, fmt.Sprintf("%s:server", providerPkg), "api")
	if err != nil {
		log.Fatalf("failed to get Localstack server endpoint: %v", err)
	}

	instance := &localstack.ClusterInstance{
		Endpoint: endpoint,
	}

	p.EmitResult(instance)
}
