// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
	"namespacelabs.dev/foundation/framework/resources"
	postgrespb "namespacelabs.dev/foundation/library/database/postgres"
)

// Connect to a Postgres Database resource.
func ConnectToResource(ctx context.Context, res *resources.Parsed, resourceRef string, opts NewDBOptions) (*DB, error) {
	db := &postgrespb.DatabaseInstance{}
	if err := res.Unmarshal(resourceRef, db); err != nil {
		return nil, err
	}

	config, err := pgxpool.ParseConfig(db.ConnectionUri)
	if err != nil {
		return nil, err
	}

	// Only connect when the pool starts to be used.
	config.LazyConnect = true

	maxConnIdleTime := time.Minute
	if db.GetMaxConnIdleTime() != "" {
		dur, err := time.ParseDuration(db.GetMaxConnIdleTime())
		if err != nil {
			return nil, err
		}
		maxConnIdleTime = dur
	}

	config.MaxConnIdleTime = maxConnIdleTime

	conn, err := pgxpool.ConnectConfig(ctx, config)
	if err != nil {
		return nil, err
	}

	return NewDB(db, conn, opts), nil
}
