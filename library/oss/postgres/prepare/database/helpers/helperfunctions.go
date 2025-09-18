// Copyright 2022, 2025 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package helpers

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/jackc/pgx/v5"
	"namespacelabs.dev/foundation/framework/rpcerrors/multierr"
	"namespacelabs.dev/foundation/library/oss/postgres"
	"namespacelabs.dev/foundation/schema"
	universepg "namespacelabs.dev/foundation/universe/db/postgres"
)

const (
	connBackoff = 1500 * time.Millisecond
	connTimeout = 5 * time.Minute
)

type helperFunction struct {
	provisionSql string
	cleanupSql   string
}

type DatabaseIntent interface {
	GetAutoRemoveHelperFunctions() bool
	GetProvisionHelperFunctions() bool
	GetSchema() []*schema.FileContents
}

// Collection of optional helper functions
// To provision these functions add
//   provision_helper_functions: true
// to the database intent.

func allHelperFunctions() []helperFunction {
	// If removing a helper function, keep the cleanupSql.
	return []helperFunction{
		// fn_ensure_table is a lock-friendly replacement for 'CREATE TABLE IF NOT EXISTS'.
		// WARNING: This function translates all names into lowercase (as plain postgres would).
		// If you want to use lowercase characters, (e.g. through quotation) do not use this function.
		//
		// Example usage:
		//
		// SELECT fn_ensure_table('testtable', $$
		//   UserID TEXT NOT NULL,
		//   PRIMARY KEY(UserID)
		// $$);
		helperFunction{
			provisionSql: `
CREATE OR REPLACE FUNCTION fn_ensure_table(tname TEXT, def TEXT)
  RETURNS void
  LANGUAGE plpgsql AS
$func$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_tables
    WHERE schemaname = 'public' AND tablename = LOWER(tname)
  ) THEN
    EXECUTE 'CREATE TABLE IF NOT EXISTS ' || tname || ' (' || def || ');';
  END IF;
END
$func$;
`,
			cleanupSql: `
DROP FUNCTION IF EXISTS fn_ensure_table(tname TEXT, def TEXT);
`,
		},

		// fn_ensure_column is a lock-friendly replacement for `ALTER TABLE ... ADD COLUMN IF NOT EXISTS`.
		// WARNING: This function translates all names into lowercase (as plain postgres would).
		// If you want to use lowercase characters, (e.g. through quotation) do not use this function.
		//
		// Example usage:
		//
		// SELECT fn_ensure_column('testtable', 'CreatedAt', 'TIMESTAMP DEFAULT CURRENT_TIMESTAMP');
		helperFunction{
			provisionSql: `
CREATE OR REPLACE FUNCTION fn_ensure_column(tname TEXT, cname TEXT, def TEXT)
  RETURNS void
  LANGUAGE plpgsql AS
$func$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_name = LOWER(tname) AND column_name = LOWER(cname)
  ) THEN
    EXECUTE 'ALTER TABLE ' || tname || ' ADD COLUMN IF NOT EXISTS ' || cname || ' ' || def || ';';
  END IF;
END
$func$;
`,
			cleanupSql: `
DROP FUNCTION IF EXISTS fn_ensure_column(tname TEXT, cname TEXT, def TEXT);
`,
		},

		// fn_ensure_column_not_exists is a lock-friendly replacement for `ALTER TABLE ... DROP COLUMN IF EXISTS`.
		// WARNING: This function translates all names into lowercase (as plain postgres would).
		// If you want to use lowercase characters, (e.g. through quotation) do not use this function.
		//
		// Example usage:
		//
		// SELECT fn_ensure_column_not_exists('testtable', 'CreatedAt');
		helperFunction{
			provisionSql: `
CREATE OR REPLACE FUNCTION fn_ensure_column_not_exists(tname TEXT, cname TEXT)
  RETURNS void
  LANGUAGE plpgsql AS
$func$
BEGIN
  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_name = LOWER(tname) AND column_name = LOWER(cname)
  ) THEN
    EXECUTE 'ALTER TABLE ' || tname || ' DROP COLUMN IF EXISTS ' || cname || ';';
END IF;
END
$func$;
`,
			cleanupSql: `
DROP FUNCTION IF EXISTS fn_ensure_column_not_exists(tname TEXT, cname TEXT);
`,
		},

		// fn_ensure_column_not_null is a lock-friendly replacement for `ALTER TABLE ... ALTER COLUMN ... SET NOT NULL`.
		// WARNING: This function translates all names into lowercase (as plain postgres would).
		// If you want to use lowercase characters, (e.g. through quotation) do not use this function.
		//
		// Example usage:
		//
		// SELECT fn_ensure_column_not_null('testtable', 'Role');
		helperFunction{
			provisionSql: `
CREATE OR REPLACE FUNCTION fn_ensure_column_not_null(tname TEXT, cname TEXT)
  RETURNS void
  LANGUAGE plpgsql AS
$func$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_name = LOWER(tname) AND column_name = LOWER(cname) AND is_nullable = 'NO'
  ) THEN
    EXECUTE 'ALTER TABLE ' || tname || ' ALTER COLUMN ' || cname || ' SET NOT NULL;';
  END IF;
END
$func$;
`,
			cleanupSql: `
DROP FUNCTION IF EXISTS fn_ensure_column_not_null(tname TEXT, cname TEXT);
`,
		},

		// fn_ensure_column_nullable is a lock-friendly replacement for `ALTER TABLE ... ALTER COLUMN ... DROP NOT NULL`.
		// WARNING: This function translates all names into lowercase (as plain postgres would).
		// If you want to use lowercase characters, (e.g. through quotation) do not use this function.
		//
		// Example usage:
		//
		// SELECT fn_ensure_column_nullable('testtable', 'Role');
		helperFunction{
			provisionSql: `
CREATE OR REPLACE FUNCTION fn_ensure_column_nullable(tname TEXT, cname TEXT)
  RETURNS void
  LANGUAGE plpgsql AS
$func$
BEGIN
  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_name = LOWER(tname) AND column_name = LOWER(cname) AND is_nullable = 'NO'
  ) THEN
    EXECUTE 'ALTER TABLE ' || tname || ' ALTER COLUMN ' || cname || ' DROP NOT NULL;';
END IF;
END
$func$;
`,
			cleanupSql: `
DROP FUNCTION IF EXISTS fn_ensure_column_nullable(tname TEXT, cname TEXT);
`,
		},

		// fn_ensure_replica_identity is a lock-friendly replacement for `ALTER TABLE ... REPLICA IDENTITY ...`.
		// WARNING: This function translates all names into lowercase (as plain postgres would).
		// If you want to use lowercase characters, (e.g. through quotation) do not use this function.
		// Does not support index identities.
		//
		// Example usage:
		//
		// SELECT fn_ensure_replica_identity('testtable', 'FULL');

		helperFunction{
			provisionSql: `
CREATE OR REPLACE FUNCTION fn_ensure_replica_identity(tname TEXT, replident TEXT)
  RETURNS void
  LANGUAGE plpgsql AS
$func$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_class WHERE oid = tname::regclass AND CASE relreplident
          WHEN 'd' THEN 'default'
          WHEN 'n' THEN 'nothing'
          WHEN 'f' THEN 'full'
       END = LOWER(replident)
  ) THEN
    EXECUTE 'ALTER TABLE ' || tname || ' REPLICA IDENTITY ' || replident || ';';
  END IF;
END
$func$;
`,
			cleanupSql: `
DROP FUNCTION IF EXISTS fn_ensure_replica_identity(tname TEXT, replident TEXT);
`,
		},
	}
}

func helpersProvisionSql() string {
	var sb strings.Builder

	helpers := allHelperFunctions()
	for _, h := range helpers {
		sb.WriteString(h.provisionSql)
		sb.WriteString("\n")
	}

	return sb.String()
}

func helpersCleanupSql() string {
	var sb strings.Builder

	helpers := allHelperFunctions()
	for i := len(helpers) - 1; i >= 0; i-- {
		sb.WriteString(helpers[i].cleanupSql)
		sb.WriteString("\n")
	}

	return sb.String()
}

type backOff struct {
	interval time.Duration
	deadline time.Time
	jitter   time.Duration
}

func (b backOff) Reset() {}
func (b backOff) NextBackOff() time.Duration {
	if time.Now().After(b.deadline) {
		return backoff.Stop
	}
	return b.interval - b.jitter/2 + time.Duration(rand.Int63n(int64(b.jitter)))
}

func ApplyWithCleanup(ctx context.Context, intent DatabaseIntent, db *universepg.DB) (reserr error) {
	if intent.GetAutoRemoveHelperFunctions() {
		defer func() {
			if err := applyWithRetry(ctx, db, helpersCleanupSql()); err != nil {
				reserr = multierr.New(reserr, fmt.Errorf("unable to clean up helper functions: %w", err))
			}
		}()
	}

	if intent.GetProvisionHelperFunctions() {
		if err := applyWithRetry(ctx, db, helpersProvisionSql()); err != nil {
			return fmt.Errorf("unable to apply helper functions: %w", err)
		}
	}

	for _, oneSchema := range intent.GetSchema() {
		if err := applyWithRetry(ctx, db, string(oneSchema.Contents)); err != nil {
			return fmt.Errorf("unable to apply schema %q: %w", oneSchema.Path, err)
		}
	}

	return nil
}

func applyWithRetry(ctx context.Context, db *universepg.DB, sql string) error {
	return backoff.Retry(func() error {
		_, err := db.Exec(ctx, sql)

		if !universepg.ErrorIsRetryable(err) {
			return backoff.Permanent(err)
		}

		return err
	}, backOff{
		interval: 100 * time.Millisecond,
		deadline: time.Now().Add(15 * time.Second),
		jitter:   100 * time.Millisecond,
	})
}

func EnsureDatabase(ctx context.Context, cluster postgres.ClusterInstance, name string, extraArgs string) (bool, error) {
	// Postgres needs a database to connect to so we pin one that is guaranteed to exist.
	postgresConn, err := connect(ctx, cluster, "postgres")
	if err != nil {
		return false, err
	}
	defer func() {
		if err := postgresConn.Close(ctx); err != nil {
			log.Printf("unable to close database connection: %v", err)
		}
	}()

	exists, err := existsDatabase(ctx, postgresConn, name)
	if err != nil {
		return false, err
	}

	if !exists {
		// SQL arguments can only be values, not identifiers.
		// https://www.postgresql.org/docs/9.5/xfunc-sql.html
		// `existsDb` already uses the database name as an SQL argument, so we already passed its validation.
		// Still, let's do some basic sanity checking (whitespaces are forbidden), as we need to use Sprintf here.
		// Valid database names are defined at https://www.postgresql.org/docs/current/sql-syntax-lexical.html#SQL-SYNTAX-IDENTIFIERS
		if len(strings.Fields(name)) > 1 || strings.Contains(name, "-") {
			return false, fmt.Errorf("invalid database name: %s", name)
		}

		if _, err := postgresConn.Exec(ctx, fmt.Sprintf("CREATE DATABASE \"%s\" %s;", name, extraArgs)); err != nil {
			return false, fmt.Errorf("failed to create database %q: %w", name, err)
		}
	}

	return exists, err
}

func existsDatabase(ctx context.Context, conn *pgx.Conn, name string) (bool, error) {
	rows, err := conn.Query(ctx, "SELECT FROM pg_database WHERE datname = $1;", name)
	if err != nil {
		return false, fmt.Errorf("failed to check for database %q: %w", name, err)
	}
	defer rows.Close()

	return rows.Next(), nil
}

func connect(ctx context.Context, cluster postgres.ClusterInstance, db string) (*pgx.Conn, error) {
	cfg, err := pgx.ParseConfig(postgres.ConnectionUri(cluster, db))
	if err != nil {
		return nil, err
	}
	cfg.ConnectTimeout = connBackoff

	ctx, cancel := context.WithTimeout(ctx, connTimeout)
	defer cancel()

	// Retry until backend is ready.
	return backoff.RetryWithData(func() (*pgx.Conn, error) {
		conn, err := pgx.ConnectConfig(ctx, cfg)
		if err == nil {
			return conn, nil
		}

		log.Printf("failed to connect to postgres: %v\n", err)
		return nil, err
	}, backoff.WithContext(backoff.NewConstantBackOff(connBackoff), ctx))
}
