// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package postgres

import (
	"context"
	"os"
	"strconv"
	"time"

	"namespacelabs.dev/foundation/framework/resources"
	"namespacelabs.dev/foundation/internal/fnerrors"
	postgrespb "namespacelabs.dev/foundation/library/database/postgres"
	"namespacelabs.dev/foundation/library/oss/postgres"
)

func ProvideDatabase(ctx context.Context, db *DatabaseArgs, deps ExtensionDeps) (*DB, error) {
	if db.ResourceRef == "" {
		return nil, fnerrors.Newf("resourceRef is required")
	}

	if db.GetMaxConns() > 0 && db.GetMaxConnsFromEnv() != "" {
		return nil, fnerrors.Newf("maxConns and maxConnsFromEnv cannot both be set")
	}

	res, err := resources.LoadResources()
	if err != nil {
		return nil, err
	}

	tp, err := deps.OpenTelemetry.GetTracerProvider()
	if err != nil {
		return nil, err
	}

	overrides := &ConfigOverrides{
		MaxConns: db.GetMaxConns(),
	}

	if db.GetMaxConnsFromEnv() != "" {
		parsed, err := strconv.ParseInt(os.Getenv(db.GetMaxConnsFromEnv()), 10, 32)
		if err != nil {
			return nil, fnerrors.Newf("%s is not a valid number: %w", db.GetMaxConnsFromEnv(), err)
		}

		overrides.MaxConns = int32(parsed)
	}

	if db.GetMaxConnsIdleTime() != nil {
		overrides.MaxConnIdleTime = db.GetMaxConnsIdleTime().AsDuration()
	}

	if db.GetIdleInTransactionSessionTimeoutMs() > 0 {
		overrides.IdleInTransactionSessionTimeout = time.Millisecond * time.Duration(db.GetIdleInTransactionSessionTimeoutMs())
	}

	return ConnectToResource(ctx, res, db.ResourceRef, tp, db.GetClient(), overrides)
}

// Workaround the fact that foundation doesn't know about primitive types.
type ConnUri string

func ProvideDatabaseReference(ctx context.Context, args *DatabaseReferenceArgs, deps ExtensionDeps) (ConnUri, error) {
	if args.ClusterRef == "" {
		return "", fnerrors.Newf("clusterRef is required")
	}

	if args.Database == "" {
		return "", fnerrors.Newf("database is required")
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
