// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package base

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
	"go.opentelemetry.io/otel/trace"
	"namespacelabs.dev/foundation/std/go/core"
	"namespacelabs.dev/foundation/universe/db/postgres"
)

type WireDatabase struct {
	ready          core.Check
	tracerProvider trace.TracerProvider
}

func logf(message string, args ...interface{}) {
	fmt.Fprintf(os.Stdout, "%s : %s\n", time.Now().String(), fmt.Sprintf(message, args...))
}

func (w WireDatabase) ProvideDatabase(ctx context.Context, db *postgres.Database) (*postgres.DB, error) {
	if db.Credentials.User.GetFromPath() != "" || db.Credentials.Password.GetFromPath() != "" {
		return nil, fmt.Errorf("user and password secrets should already be resolved during provide")
	}

	// Config has to be created by ParseConfig
	config, err := pgxpool.ParseConfig(fmt.Sprintf("postgres://%s:%s@%s:%d/%s",
		db.Credentials.User.GetValue(),
		db.Credentials.Password.GetValue(),
		db.HostedAt.Address,
		db.HostedAt.Port,
		db.Name))
	if err != nil {
		return nil, err
	}

	// Only connect when the pool starts to be used.
	config.LazyConnect = true

	conn, err := pgxpool.ConnectConfig(ctx, config)
	if err != nil {
		logf("failed to connect: %v", err)
		return nil, err
	}

	// Asynchronously wait until a database connection is ready.
	w.ready.RegisterFunc(fmt.Sprintf("%s/%s", core.InstantiationPathFromContext(ctx), db.Name), func(ctx context.Context) error {
		return conn.Ping(ctx)
	})

	var tracer trace.Tracer
	if w.tracerProvider != nil {
		tracer = w.tracerProvider.Tracer(Package__26debk.PackageName)
	}

	return postgres.NewDB(conn, tracer), nil
}

func ProvideWireDatabase(_ context.Context, _ *WireDatabaseArgs, deps ExtensionDeps) (WireDatabase, error) {
	t, err := deps.OpenTelemetry.GetTracerProvider()
	if err != nil {
		return WireDatabase{}, err
	}

	return WireDatabase{deps.ReadinessCheck, t}, nil
}
