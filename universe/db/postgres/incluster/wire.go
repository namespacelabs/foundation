// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package incluster

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"

	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/universe/db/postgres"
	"namespacelabs.dev/foundation/universe/db/postgres/internal/base"
	"namespacelabs.dev/foundation/universe/db/postgres/internal/gencreds"
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

	return ProvideDb(ctx, db.Name, endpoint, deps.Creds, deps.Wire)
}

func ProvideDb(ctx context.Context, name string, endpoint *schema.Endpoint, creds *gencreds.Creds, wire base.WireDatabase) (*postgres.DB, error) {
	base := &postgres.Database{
		Name: name,
		HostedAt: &postgres.Endpoint{
			Address: endpoint.AllocatedName,
			Port:    uint32(endpoint.Port.ContainerPort),
		},
	}

	return wire.ProvideDatabase(ctx, base, "postgres", creds.Password)
}
