// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package main

import (
	"fmt"
	"log"

	"namespacelabs.dev/foundation/framework/resources"
	"namespacelabs.dev/foundation/framework/resources/provider"
	redisclass "namespacelabs.dev/foundation/library/database/redis"
	redisprovider "namespacelabs.dev/foundation/library/oss/redis"
)

const providerPkg = "namespacelabs.dev/foundation/library/oss/redis"

func main() {
	_, p := provider.MustPrepare[*redisprovider.ClusterIntent]()

	endpoint, err := resources.LookupServerEndpoint(p.Resources, fmt.Sprintf("%s:server", providerPkg), "redis")
	if err != nil {
		log.Fatalf("failed to get redis server endpoint: %v", err)
	}

	password, err := resources.ReadSecret(p.Resources, fmt.Sprintf("%s:password", providerPkg))
	if err != nil {
		log.Fatalf("failed to read redis password: %v", err)
	}

	instance := &redisclass.ClusterInstance{
		Address:  endpoint,
		Password: string(password),
	}

	p.EmitResult(instance)
}
