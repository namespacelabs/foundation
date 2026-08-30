// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package main

import (
	"fmt"
	"log"

	"namespacelabs.dev/foundation/framework/resources"
	"namespacelabs.dev/foundation/framework/resources/provider"
	cockroachclass "namespacelabs.dev/foundation/library/database/cockroach"
	"namespacelabs.dev/foundation/library/oss/cockroach"
)

const (
	providerPkg = "namespacelabs.dev/foundation/library/oss/cockroach"
	user        = "postgres"
)

func main() {
	_, p := provider.MustPrepare[*cockroach.ClusterIntent]()

	endpoint, err := resources.LookupServerEndpoint(p.Resources, fmt.Sprintf("%s:server", providerPkg), "postgres")
	if err != nil {
		log.Fatalf("failed to get cockroach server endpoint: %v", err)
	}

	password, err := resources.ReadSecret(p.Resources, fmt.Sprintf("%s:password", providerPkg))
	if err != nil {
		log.Fatalf("failed to read cockroach password: %v", err)
	}

	instance := &cockroachclass.ClusterInstance{
		Address:  endpoint,
		User:     user,
		Password: string(password),
	}

	p.EmitResult(instance)
}
