// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package main

import (
	"fmt"
	"log"
	"time"

	"namespacelabs.dev/foundation/framework/resources"
	"namespacelabs.dev/foundation/framework/resources/provider"
	postgresclass "namespacelabs.dev/foundation/library/database/postgres"
)

const (
	providerPkg = "namespacelabs.dev/foundation/library/oss/postgres"
	connBackoff = 500 * time.Millisecond
)

func main() {
	intent := &postgresclass.DatabaseIntent{}
	_, r := provider.MustPrepare(intent)

	endpoint, err := resources.LookupServerEndpoint(r, fmt.Sprintf("%s:postgresServer", providerPkg), "postgres")
	if err != nil {
		log.Fatalf("failed to get postgres server endpoint: %v", err)
	}

	instance := &postgresclass.DatabaseInstance{
		Name:     intent.Name,
		Url:      endpoint,
		Password: "", // TODO
	}

	// TODO apply schema

	provider.EmitResult(instance)
}
