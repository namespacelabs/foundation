// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package postgres

import (
	"context"

	"namespacelabs.dev/foundation/framework/resources"
	"namespacelabs.dev/foundation/internal/fnerrors"
	postgrespb "namespacelabs.dev/foundation/library/database/postgres"
	"namespacelabs.dev/foundation/library/oss/postgres"
)

func ProvideDatabase(ctx context.Context, db *DatabaseArgs, deps ExtensionDeps) (*DB, error) {
	if db.ResourceRef == "" {
		return nil, fnerrors.New("resourceRef is required")
	}

	res, err := resources.LoadResources()
	if err != nil {
		return nil, err
	}

	tp, err := deps.OpenTelemetry.GetTracerProvider()
	if err != nil {
		return nil, err
	}

	return ConnectToResource(ctx, res, db.ResourceRef, tp, &ConfigOverrides{
		MaxConns: db.MaxConns,
	})
}

// Workaround the fact that foundation doesn't know about primitive types.
type ConnUri string

func ProvideDatabaseReference(ctx context.Context, args *DatabaseReferenceArgs, deps ExtensionDeps) (ConnUri, error) {
	if args.ClusterRef == "" {
		return "", fnerrors.New("clusterRef is required")
	}

	if args.Database == "" {
		return "", fnerrors.New("database is required")
	}

	res, err := resources.LoadResources()
	if err != nil {
		return "", err
	}

	cluster := &postgrespb.ClusterInstance{}
	if err := res.Unmarshal(args.ClusterRef, cluster); err != nil {
		return "", err
	}

	return ConnUri(postgres.ConnectionUri(cluster, args.Database)), nil
}
