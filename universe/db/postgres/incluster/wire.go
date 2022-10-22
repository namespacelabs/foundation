// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package incluster

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"

	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/universe/db/postgres"
)

const EndpointFlag = "postgresql_endpoint"

var (
	postgresqlEndpoint = flag.String(EndpointFlag, "", "Endpoint configuration.")
)

func GetEndpoint() (*schema.Endpoint, error) {
	if *postgresqlEndpoint == "" {
		return nil, nil
	}

	var endpoint schema.Endpoint
	if err := json.Unmarshal([]byte(*postgresqlEndpoint), &endpoint); err != nil {
		return nil, fmt.Errorf("failed to parse postgresql endpoint configuration: %w", err)
	}

	return &endpoint, nil
}

func ProvideDatabase(ctx context.Context, db *Database, deps ExtensionDeps) (*postgres.DB, error) {
	endpoint, err := GetEndpoint()
	if err != nil {
		return nil, err
	}

	if endpoint == nil {
		return nil, fmt.Errorf("startup configuration missing, --%s not specified", EndpointFlag)
	}

	return deps.Wire.ProvideDatabase(ctx, &postgres.Database{
		Name: db.Name,
		HostedAt: &postgres.Database_Endpoint{
			Address: endpoint.AllocatedName,
			Port:    uint32(endpoint.Port.ContainerPort),
		},
		Credentials: &postgres.Database_Credentials{
			User:     &postgres.Database_Credentials_Secret{Value: "postgres"},
			Password: &postgres.Database_Credentials_Secret{Value: deps.Creds.Password},
		},
	})
}
