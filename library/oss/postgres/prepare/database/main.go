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
	postgresclass "namespacelabs.dev/foundation/library/database/postgres"
	"namespacelabs.dev/foundation/library/oss/postgres"
)

const (
	providerPkg = "namespacelabs.dev/foundation/library/oss/postgres"
	connBackoff = 500 * time.Millisecond
	user        = "postgres"
)

func main() {
	ctx, p := provider.MustPrepare[*postgres.DatabaseIntent]()

	cluster := &postgresclass.ClusterInstance{}
	if err := p.Resources.Unmarshal(fmt.Sprintf("%s:cluster", providerPkg), cluster); err != nil {
		log.Fatalf("unable to read required resource \"cluster\": %v", err)
	}

	conn, err := ensureDatabase(ctx, cluster, p.Intent.Name)
	if err != nil {
		log.Fatalf("unable to create database %q: %v", p.Intent.Name, err)
	}
	defer conn.Close(ctx)

	for _, schema := range p.Intent.Schema {
		if _, err = conn.Exec(ctx, string(schema.Contents)); err != nil {
			log.Fatalf("unable to apply schema %q: %v", schema.Path, err)
		}
	}

	instance := &postgresclass.DatabaseInstance{
		ConnectionUri:  connectionUri(cluster, p.Intent.Name),
		Name:           p.Intent.Name,
		User:           user,
		Password:       cluster.Password,
		ClusterAddress: cluster.Address,
	}

	p.EmitResult(instance)
}

func ensureDatabase(ctx context.Context, cluster *postgresclass.ClusterInstance, name string) (*pgx.Conn, error) {
	// Postgres needs a database to connect to so we pin one that is guaranteed to exist.
	conn, err := connect(ctx, cluster, "postgres")
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	exists, err := existsDatabase(ctx, conn, name)
	if err != nil {
		return nil, err
	}

	if !exists {
		// SQL arguments can only be values, not identifiers.
		// https://www.postgresql.org/docs/9.5/xfunc-sql.html
		// `existsDb` already uses the database name as an SQL argument, so we already passed its validation.
		// Still, let's do some basic sanity checking (whitespaces are forbidden), as we need to use Sprintf here.
		// Valid database names are defined at https://www.postgresql.org/docs/current/sql-syntax-lexical.html#SQL-SYNTAX-IDENTIFIERS
		if len(strings.Fields(name)) > 1 || strings.Contains(name, "-") {
			return nil, fmt.Errorf("invalid database name: %s", name)
		}

		if _, err := conn.Exec(ctx, fmt.Sprintf("CREATE DATABASE \"%s\";", name)); err != nil {
			return nil, fmt.Errorf("failed to create database %q: %w", name, err)
		}
	}

	return connect(ctx, cluster, name)
}

func existsDatabase(ctx context.Context, conn *pgx.Conn, name string) (bool, error) {
	rows, err := conn.Query(ctx, "SELECT FROM pg_database WHERE datname = $1;", name)
	if err != nil {
		return false, fmt.Errorf("failed to check for database %q: %w", name, err)
	}
	defer rows.Close()

	return rows.Next(), nil
}

func connect(ctx context.Context, cluster *postgresclass.ClusterInstance, db string) (conn *pgx.Conn, err error) {
	cfg, err := pgx.ParseConfig(connectionUri(cluster, db))
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

func connectionUri(cluster *postgresclass.ClusterInstance, db string) string {
	return fmt.Sprintf("postgres://%s:%s@%s/%s", user, cluster.Password, cluster.Address, db)
}
