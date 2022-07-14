// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package rds

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"

	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/universe/db/postgres"
)

var (
	postgresqlEndpoint = flag.String("postgresql_endpoint", "", "Endpoint configuration.")
)

func getEndpoint() (*schema.Endpoint, error) {
	if *postgresqlEndpoint == "" {
		return nil, errors.New("startup configuration missing, --postgresql_endpoint not specified")
	}

	var endpoint schema.Endpoint
	if err := json.Unmarshal([]byte(*postgresqlEndpoint), &endpoint); err != nil {
		return nil, fmt.Errorf("failed to parse postgresql endpoint configuration: %w", err)
	}

	return &endpoint, nil
}

func ProvideDatabase(ctx context.Context, db *Database, deps ExtensionDeps) (*postgres.DB, error) {
	endpoint, err := getEndpoint()
	if err != nil {
		return nil, err
	}

	base := &postgres.Database{
		Name:       db.Name,
		SchemaFile: db.SchemaFile,
		HostedAt: &postgres.Endpoint{
			Address: endpoint.AllocatedName,
			Port:    uint32(endpoint.Port.ContainerPort),
		},
	}

	return deps.Wire.ProvideDatabase(ctx, base, "postgres", deps.Creds.Password)
}
