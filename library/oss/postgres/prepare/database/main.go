// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/jackc/pgx/v4"
	"namespacelabs.dev/foundation/framework/resources/provider"
	"namespacelabs.dev/foundation/library/database/postgres"
)

const (
	providerPkg = "namespacelabs.dev/foundation/library/oss/postgres"
	connBackoff = 500 * time.Millisecond
)

func main() {
	intent := &postgres.DatabaseIntent{}
	ctx, r := provider.MustPrepare(intent)

	cluster := &postgres.ClusterInstance{}
	resource := fmt.Sprintf("%s:cluster", providerPkg)
	if err := r.Unmarshal(resource, cluster); err != nil {
		log.Fatalf("unable to read required resource %q: %v", resource, err)
	}

	conn, err := ensureDb(ctx, cluster.Url, cluster.Password, intent.Name)
	if err != nil {
		log.Fatalf("unable to create database %q: %v", intent.Name, err)
	}
	defer conn.Close(ctx)

	for _, schema := range intent.Schema {
		if _, err = conn.Exec(ctx, string(schema.Contents)); err != nil {
			log.Fatalf("unable to apply schema %q: %v", schema.Path, err)
		}
	}

	instance := &postgres.DatabaseInstance{
		Name:     intent.Name,
		Url:      cluster.Url,
		Password: cluster.Password,
	}

	provider.EmitResult(instance)
}

func ensureDb(ctx context.Context, password, url, name string) (*pgx.Conn, error) {
	// Postgres needs a database to connect to so we pin one that is guaranteed to exist.
	conn, err := connect(ctx, password, url, "postgres")
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	exists, err := existsDb(ctx, conn, name)
	if err != nil {
		return nil, err
	}

	if !exists {
		// SQL arguments can only be values, not identifiers.
		// https://www.postgresql.org/docs/9.5/xfunc-sql.html
		// `existsDb` already uses the database name as an SQL argument, so we already passed its validation.
		// Still, let's do some basic sanity checking (whitespaces are forbidden), as we need to use Sprintf here.
		if len(strings.Fields(name)) > 1 {
			return nil, fmt.Errorf("invalid database name: %s", name)
		}

		if _, err := conn.Exec(ctx, fmt.Sprintf("CREATE DATABASE %s;", name)); err != nil {
			return nil, fmt.Errorf("failed to create database %q: %w", name, err)
		}
	}

	return connect(ctx, password, url, name)
}

func existsDb(ctx context.Context, conn *pgx.Conn, name string) (bool, error) {
	rows, err := conn.Query(ctx, "SELECT FROM pg_database WHERE datname = $1;", name)
	if err != nil {
		return false, fmt.Errorf("failed to check for database %q: %w", name, err)
	}
	defer rows.Close()

	return rows.Next(), nil
}

func connect(ctx context.Context, password, url, db string) (conn *pgx.Conn, err error) {
	cfg, err := pgx.ParseConfig(fmt.Sprintf("postgres://postgres:%s@%s/%s", password, url, db))
	if err != nil {
		return nil, err
	}
	cfg.ConnectTimeout = connBackoff

	// Retry until backend is ready.
	err = backoff.Retry(func() error {
		conn, err = pgx.ConnectConfig(ctx, cfg)
		if err == nil {
			return nil
		}

		log.Printf("failed to connect to postgres: %v\n", err)
		return err
	}, backoff.WithContext(backoff.NewConstantBackOff(connBackoff), ctx))

	return conn, err
}
