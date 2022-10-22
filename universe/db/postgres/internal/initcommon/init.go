// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package initcommon

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/dustin/go-humanize"
	"github.com/jackc/pgx/v4"
	"namespacelabs.dev/foundation/universe/db/postgres"
)

const connBackoff = 1 * time.Second

func existsDb(ctx context.Context, conn *pgx.Conn, dbName string) (bool, error) {
	rows, err := conn.Query(ctx, "SELECT FROM pg_database WHERE datname = $1;", dbName)
	if err != nil {
		return false, fmt.Errorf("failed to check for database %s: %w", dbName, err)
	}
	defer rows.Close()

	return rows.Next(), nil
}

func connect(ctx context.Context, user string, password string, address string, port uint32, db string) (conn *pgx.Conn, err error) {
	connString := fmt.Sprintf("postgres://%s:%s@%s:%d/%s", user, password, address, port, db)
	count := 0
	err = backoff.Retry(func() error {
		addrPort := fmt.Sprintf("%s:%d", address, port)

		// Use a more aggressive connect to determine whether the server already
		// has an open serving port. If it does, we then defer to pgx.Connect to
		// take as much time as it needs.
		rawConn, err := net.DialTimeout("tcp", addrPort, 3*connBackoff)
		if err != nil {
			log.Printf("Failed to tcp dial %s: %v", addrPort, err)
			return err
		}

		rawConn.Close()

		count++
		log.Printf("Connecting to postgres (%s try), address is `%s:%d`.", humanize.Ordinal(count), address, port)
		conn, err = pgx.Connect(ctx, connString)
		if err != nil {
			log.Printf("Failed to connect to postgres: %v", err)
		}
		return err
	}, backoff.WithContext(backoff.NewConstantBackOff(connBackoff), ctx))

	if err != nil {
		return nil, fmt.Errorf("unable to establish postgres connection: %w", err)
	}

	return conn, nil
}

func ensureDb(ctx context.Context, conn *pgx.Conn, db *postgres.Database) error {
	// Postgres does not support CREATE DATABASE IF NOT EXISTS
	log.Printf("Querying for existing databases.")
	exists, err := existsDb(ctx, conn, db.Name)
	if err != nil {
		return err
	}

	if exists {
		log.Printf("Database `%s` already exists.", db.Name)
		return nil
	}

	// SQL arguments can only be values, not identifiers.
	// https://www.postgresql.org/docs/9.5/xfunc-sql.html
	// As we need to use Sprintf instead, let's do some basic sanity checking (whitespaces are forbidden).
	if len(strings.Fields(db.Name)) > 1 {
		return fmt.Errorf("invalid database name: %s", db.Name)
	}

	if _, err := conn.Exec(ctx, fmt.Sprintf("CREATE DATABASE %s;", db.Name)); err != nil {
		return fmt.Errorf("failed to create database `%s`: %w", db.Name, err)
	}

	log.Printf("Created database `%s`.", db.Name)
	return nil
}

func applySchema(ctx context.Context, conn *pgx.Conn, db *postgres.Database) error {
	schema, err := os.ReadFile(db.SchemaFile.Path)
	if err != nil {
		return fmt.Errorf("unable to read file %s: %v", db.SchemaFile.Path, err)
	}

	log.Printf("Applying schema %s.", db.SchemaFile.Path)
	_, err = conn.Exec(ctx, string(schema))
	if err != nil {
		return fmt.Errorf("unable to execute schema %s: %v", db.SchemaFile.Path, err)
	}
	return nil
}

func readSecret(secret *postgres.Database_Credentials_Secret, cache map[string]string) error {
	if secret.Value != "" {
		if secret.FromPath != "" {
			return fmt.Errorf("value and from_path may not both be set")
		}
		return nil
	}

	if val, ok := cache[secret.FromPath]; ok {
		secret.Value = val
		return nil
	}

	bytes, err := os.ReadFile(secret.FromPath)
	if err != nil {
		return fmt.Errorf("unable to read file %s: %w", secret.FromPath, err)
	}

	secret.Value = string(bytes)
	secret.FromPath = "" // Unset path now that we read the content.
	cache[secret.FromPath] = secret.Value

	return nil
}

func readCreds(dbs []*postgres.Database) error {
	cache := map[string]string{}

	for _, db := range dbs {
		if err := readSecret(db.Credentials.Password, cache); err != nil {
			return err
		}

		if db.Credentials.User.GetFromPath() == "" && db.Credentials.GetUser().GetValue() == "" {
			db.Credentials.User = &postgres.Database_Credentials_Secret{
				Value: "postgres", // default user
			}
		} else {
			if err := readSecret(db.Credentials.User, cache); err != nil {
				return err
			}
		}
	}

	return nil
}

func ReadConfigs() ([]*postgres.Database, error) {
	dbs := []*postgres.Database{}

	for _, path := range flag.Args() {
		file, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("unable to read file %s: %v", path, err)
		}

		db := &postgres.Database{}
		if err := json.Unmarshal(file, db); err != nil {
			return nil, err
		}

		dbs = append(dbs, db)
	}

	if err := readCreds(dbs); err != nil {
		return nil, err
	}

	return dbs, nil
}

func PrepareDatabase(ctx context.Context, db *postgres.Database) error {
	// Postgres needs a db to connect to so we pin one that is guaranteed to exist.
	postgresDB, err := connect(ctx, db.Credentials.User.Value, db.Credentials.Password.Value, db.HostedAt.Address, db.HostedAt.Port, "postgres")
	if err != nil {
		return err
	}
	defer postgresDB.Close(ctx)

	if err := ensureDb(ctx, postgresDB, db); err != nil {
		return err
	}

	conn, err := connect(ctx, db.Credentials.User.Value, db.Credentials.Password.Value, db.HostedAt.Address, db.HostedAt.Port, db.Name)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	return applySchema(ctx, conn, db)
}
