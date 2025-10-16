// Copyright 2022 Namespace Labs Inc; All rights reserved.
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
	"namespacelabs.dev/foundation/library/oss/postgres"
	universepg "namespacelabs.dev/foundation/universe/db/postgres"
)

const (
	connBackoff = 1500 * time.Millisecond
	connTimeout = 5 * time.Minute
)

type helperFunction struct {
	provisionSql string
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
		// NOTE: This function assumes you're working with the 'public' schema.
		//
		// Example usage:
		//
		// SELECT fn_ensure_table('testtable', $$
		//   UserID TEXT NOT NULL,
		//   PRIMARY KEY(UserID)
		// $$);
		{
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
`},

		// fn_ensure_partitioned_table is a lock-friendly replacement for 'CREATE TABLE IF NOT EXISTS PARTITION BY'.
		// WARNING: This function translates all names into lowercase (as plain postgres would).
		// If you want to use lowercase characters, (e.g. through quotation) do not use this function.
		// NOTE: This function assumes you're working with the 'public' schema.
		// NOTE: Your partition key must be part of the primary key!
		//
		// Example usage:
		//
		// SELECT fn_ensure_partitioned_table('testtable', 'RANGE (CreatedAt)', $$
		//   UserID TEXT NOT NULL,
		//   CreatedAt TIMESTAMP NOT NULL,
		//   PRIMARY KEY(CreatedAt, UserID)
		// $$);
		{
			provisionSql: `
CREATE OR REPLACE FUNCTION fn_ensure_partitioned_table(tname TEXT, part TEXT, def TEXT)
  RETURNS void
  LANGUAGE plpgsql AS
$func$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_tables
    WHERE schemaname = 'public' AND tablename = LOWER(tname)
  ) THEN
    EXECUTE 'CREATE TABLE IF NOT EXISTS ' || tname || ' (' || def || ') PARTITION BY ' || part || ';';
  END IF;
END
$func$;
`},

		// fn_ensure_daily_partitions makes sure that we have partitions for today and tomorrow for a table
		// that is partitioned by timestamp, if the table currently does not have any partitions.
		// WARNING: This function translates all names into lowercase (as plain postgres would).
		// If you want to use lowercase characters, (e.g. through quotation) do not use this function.
		// NOTE: This function assumes you're working with the 'public' schema.
		//
		// Example usage:
		//
		// SELECT fn_ensure_partitions('event_log_v2', 'event_log_p_')
		// makes sure that for the partitioned table event_log_v2 we have event_log_p_{today} and event_log_p_{tomorrow}
		{
			provisionSql: `
CREATE OR REPLACE FUNCTION fn_ensure_daily_partitions(base_name TEXT, partition_prefix TEXT)
  RETURNS void
  LANGUAGE plpgsql AS
$func$
DECLARE
    partitions_count int;
    start_today timestamp;
    start_tomorrow timestamp;
    start_day_after timestamp;
    part1_name text;
    part2_name text;
BEGIN
    -- Proceed only if the parent partitioned table exists
    IF EXISTS (
        SELECT 1 FROM pg_class c
        JOIN pg_namespace n ON n.oid = c.relnamespace
        WHERE c.relname = base_name AND n.nspname = 'public'
    ) THEN
        SELECT COUNT(*) INTO partitions_count
        FROM pg_inherits i
        JOIN pg_class c_parent ON c_parent.oid = i.inhparent
        JOIN pg_namespace n_parent ON n_parent.oid = c_parent.relnamespace
        WHERE c_parent.relname = base_name AND n_parent.nspname = 'public';

        IF partitions_count = 0 THEN
            start_today := date_trunc('day', now());
            start_tomorrow := start_today + interval '1 day';
            start_day_after := start_today + interval '2 day';

            part1_name := partition_prefix || to_char(start_today, 'YYYYMMDD');
            part2_name := partition_prefix || to_char(start_tomorrow, 'YYYYMMDD');

            -- Create partition for today [today, tomorrow)
            EXECUTE format(
                'CREATE TABLE IF NOT EXISTS %I PARTITION OF %I FOR VALUES FROM (%L) TO (%L)',
                part1_name, base_name, start_today, start_tomorrow
            );

            -- Create partition for tomorrow [tomorrow, day-after)
            EXECUTE format(
                'CREATE TABLE IF NOT EXISTS %I PARTITION OF %I FOR VALUES FROM (%L) TO (%L)',
                part2_name, base_name, start_tomorrow, start_day_after
            );
        END IF;
    END IF;
END
$func$;
`},

		// fn_ensure_column is a lock-friendly replacement for `ALTER TABLE ... ADD COLUMN IF NOT EXISTS`.
		// WARNING: This function translates all names into lowercase (as plain postgres would).
		// If you want to use lowercase characters, (e.g. through quotation) do not use this function.
		// NOTE: This function assumes that the table and column name combination is unique across all schemas!
		//
		// Example usage:
		//
		// SELECT fn_ensure_column('testtable', 'CreatedAt', 'TIMESTAMP DEFAULT CURRENT_TIMESTAMP');
		{
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
`},
		// fn_ensure_column_not_exists is a lock-friendly replacement for `ALTER TABLE ... DROP COLUMN IF EXISTS`.
		// WARNING: This function translates all names into lowercase (as plain postgres would).
		// If you want to use lowercase characters, (e.g. through quotation) do not use this function.
		// NOTE: This function assumes that the table and column name combination is unique across all schemas!
		//
		// Example usage:
		//
		// SELECT fn_ensure_column_not_exists('testtable', 'CreatedAt');
		{
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
`},

		// fn_ensure_column_not_null is a lock-friendly replacement for `ALTER TABLE ... ALTER COLUMN ... SET NOT NULL`.
		// WARNING: This function translates all names into lowercase (as plain postgres would).
		// If you want to use lowercase characters, (e.g. through quotation) do not use this function.
		// NOTE: This function assumes that the table and column name combination is unique across all schemas!
		//
		// Example usage:
		//
		// SELECT fn_ensure_column_not_null('testtable', 'Role');
		{
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
`},

		// fn_ensure_column_nullable is a lock-friendly replacement for `ALTER TABLE ... ALTER COLUMN ... DROP NOT NULL`.
		// WARNING: This function translates all names into lowercase (as plain postgres would).
		// If you want to use lowercase characters, (e.g. through quotation) do not use this function.
		// NOTE: This function assumes that the table and column name combination is unique across all schemas!
		//
		// Example usage:
		//
		// SELECT fn_ensure_column_nullable('testtable', 'Role');
		{
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
`},

		// fn_ensure_replica_identity is a lock-friendly replacement for `ALTER TABLE ... REPLICA IDENTITY ...`.
		// WARNING: This function translates all names into lowercase (as plain postgres would).
		// If you want to use lowercase characters, (e.g. through quotation) do not use this function.
		// NOTE: Does not support index identities.
		//
		// Example usage:
		//
		// SELECT fn_ensure_replica_identity('testtable', 'FULL');
		{
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
`},
	}
}

type BackOff struct {
	Interval time.Duration
	Deadline time.Time
	Jitter   time.Duration
}

func (b BackOff) Reset() {}
func (b BackOff) NextBackOff() time.Duration {
	if time.Now().After(b.Deadline) {
		return backoff.Stop
	}
	return b.Interval - b.Jitter/2 + time.Duration(rand.Int63n(int64(b.Jitter)))
}

func ApplyWithHelpers(ctx context.Context, intent *postgres.DatabaseIntent, db *universepg.DB) error {
	if intent.GetAutoRemoveHelperFunctions() {
		return fmt.Errorf("automatic helper cleanup has been retired: it is not safe when used concurrently")
	}

	if intent.GetProvisionHelperFunctions() {
		for _, helper := range allHelperFunctions() {
			if err := applyWithRetry(ctx, db, helper.provisionSql); err != nil {
				return fmt.Errorf("unable to apply helper functions: %w", err)
			}
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
	}, BackOff{
		Interval: 100 * time.Millisecond,
		Deadline: time.Now().Add(15 * time.Second),
		Jitter:   100 * time.Millisecond,
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
