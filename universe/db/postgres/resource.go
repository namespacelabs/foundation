// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
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

	// XXX TODO
	// config.ConnConfig.Tracer = ...

	conn, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, err
	}

	return NewDB(db, conn, opts), nil
}
