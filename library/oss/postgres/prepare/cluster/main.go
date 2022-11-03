// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package main

import (
	"fmt"
	"log"

	"namespacelabs.dev/foundation/framework/resources"
	"namespacelabs.dev/foundation/framework/resources/provider"
	"namespacelabs.dev/foundation/library/database/postgres"
)

const providerPkg = "namespacelabs.dev/foundation/library/oss/postgres"

func main() {
	intent := &postgres.ClusterIntent{}
	_, r := provider.MustPrepare(intent)

	endpoint, err := resources.LookupServerEndpoint(r, fmt.Sprintf("%s:server", providerPkg), "postgres")
	if err != nil {
		log.Fatalf("failed to get Postgres server endpoint: %v", err)
	}

	password, err := resources.ReadSecret(r, fmt.Sprintf("%s:password", providerPkg))
	if err != nil {
		log.Fatalf("failed to read Postgres password: %v", err)
	}

	instance := &postgres.ClusterInstance{
		Url:      endpoint,
		Password: string(password),
	}

	provider.EmitResult(instance)
}
