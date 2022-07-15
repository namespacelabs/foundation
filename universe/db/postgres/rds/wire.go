// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package rds

import (
	"context"
	"fmt"

	awsrds "github.com/aws/aws-sdk-go-v2/service/rds"
	"namespacelabs.dev/foundation/universe/db/postgres"
	"namespacelabs.dev/foundation/universe/db/postgres/incluster"
	"namespacelabs.dev/foundation/universe/db/postgres/rds/internal"
)

func ProvideDatabase(ctx context.Context, db *Database, deps ExtensionDeps) (*postgres.DB, error) {
	endpoint, err := incluster.GetEndpoint()
	if err != nil {
		return nil, err
	}

	if endpoint != nil {
		return incluster.ProvideDb(ctx, db.Name, endpoint, deps.Creds, deps.Wire)
	}

	awsCfg, err := deps.ClientFactory.New(ctx)
	if err != nil {
		return nil, err
	}
	rdscli := awsrds.NewFromConfig(awsCfg)

	id := internal.ClusterIdentifier(db.Name)

	desc, err := rdscli.DescribeDBClusters(ctx, &awsrds.DescribeDBClustersInput{
		DBClusterIdentifier: &id,
	})
	if err != nil {
		return nil, err
	}

	if len(desc.DBClusters) != 1 {
		return nil, fmt.Errorf("Expected one cluster with identifier %s, got %d", id, len(desc.DBClusters))
	}

	base := &postgres.Database{
		Name: db.Name,
		HostedAt: &postgres.Endpoint{
			Address: *desc.DBClusters[0].Endpoint,
			Port:    uint32(*desc.DBClusters[0].Port),
		},
	}
	return deps.Wire.ProvideDatabase(ctx, base, "postgres", deps.Creds.Password)
}
