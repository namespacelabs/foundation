// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package postgres

import (
	"context"
	"time"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel/trace"
	"namespacelabs.dev/foundation/framework/resources"
	postgrespb "namespacelabs.dev/foundation/library/database/postgres"
)

// Connect to a Postgres Database resource.
func ConnectToResource(ctx context.Context, res *resources.Parsed, resourceRef string, tp trace.TracerProvider, overrides *ConfigOverrides) (*DB, error) {
	db := &postgrespb.DatabaseInstance{}
	if err := res.Unmarshal(resourceRef, db); err != nil {
		return nil, err
	}

	return NewDatabaseFromConnectionUri(ctx, db, db.ConnectionUri, tp, overrides)
}

type ConfigOverrides struct {
	MaxConns        int32
	MaxConnIdleTime time.Duration
}

func NewDatabaseFromConnectionUri(ctx context.Context, db *postgrespb.DatabaseInstance, connuri string, tp trace.TracerProvider, overrides *ConfigOverrides) (*DB, error) {
	config, err := pgxpool.ParseConfig(connuri)
	if err != nil {
		return nil, err
	}

	var t trace.Tracer
	if tp != nil {
		config.ConnConfig.Tracer = otelpgx.NewTracer(otelpgx.WithTracerProvider(tp))
		t = tp.Tracer("namespacelabs.dev/foundation/universe/db/postgres")
	}

	if overrides != nil {
		if overrides.MaxConns > 0 {
			config.MaxConns = overrides.MaxConns
		}

		if overrides.MaxConnIdleTime > 0 {
			config.MaxConnIdleTime = overrides.MaxConnIdleTime
		}
	}

	conn, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, err
	}

	return newDatabase(db, conn, t), nil
}
