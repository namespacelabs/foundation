// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package rds

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"

	awsrds "github.com/aws/aws-sdk-go-v2/service/rds"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/universe/db/postgres"
	"namespacelabs.dev/foundation/universe/db/postgres/rds/internal"
)

const InclusterEndpointFlag = "rds_postgresql_endpoint"

var (
	inclusterEndpoint = flag.String(InclusterEndpointFlag, "", "Endpoint configuration.")
)

func getEndpoint(ctx context.Context, db *Database, deps ExtensionDeps) (*schema.Endpoint, error) {
	if *inclusterEndpoint != "" {
		var endpoint schema.Endpoint
		if err := json.Unmarshal([]byte(*inclusterEndpoint), &endpoint); err != nil {
			return nil, fmt.Errorf("failed to parse postgresql endpoint configuration: %w", err)
		}

		return &endpoint, nil
	}

	awsCfg, err := deps.ClientFactory.New(ctx)
	if err != nil {
		return nil, err
	}
	rdscli := awsrds.NewFromConfig(awsCfg)

	id := internal.ClusterIdentifier(deps.ServerInfo.EnvName, db.Name)

	desc, err := rdscli.DescribeDBClusters(ctx, &awsrds.DescribeDBClustersInput{
		DBClusterIdentifier: &id,
	})
	if err != nil {
		return nil, err
	}

	if len(desc.DBClusters) != 1 {
		return nil, fmt.Errorf("expected one cluster with identifier %s, got %d", id, len(desc.DBClusters))
	}

	return &schema.Endpoint{
		AllocatedName: *desc.DBClusters[0].Endpoint,
		Port: &schema.Endpoint_Port{
			ContainerPort: *desc.DBClusters[0].Port,
		},
	}, nil
}

func ProvideDatabase(ctx context.Context, db *Database, deps ExtensionDeps) (*postgres.DB, error) {
	endpoint, err := getEndpoint(ctx, db, deps)
	if err != nil {
		return nil, err
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
