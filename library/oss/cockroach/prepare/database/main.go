// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"namespacelabs.dev/foundation/framework/resources/provider"
	cockroachclass "namespacelabs.dev/foundation/library/database/cockroach"
	"namespacelabs.dev/foundation/library/oss/cockroach"
	"namespacelabs.dev/foundation/library/oss/postgres"
	"namespacelabs.dev/foundation/library/oss/postgres/prepare/database/helpers"
	universepg "namespacelabs.dev/foundation/universe/db/postgres"
)

const (
	providerPkg     = "namespacelabs.dev/foundation/library/oss/cockroach"
	connIdleTimeout = 15 * time.Minute
	caCertPath      = "/tmp/ca.pem"

	migrationLeaseTimeout = 10 * time.Minute
	migrationPollInterval = 2 * time.Second
)

var log = zerolog.New(os.Stderr).With().Timestamp().Logger()

func main() {
	ctx, p := provider.MustPrepare[*cockroach.DatabaseIntent]()

	if err := run(ctx, p); err != nil {
		log.Fatal().Err(err).Send()
	}
}

func run(ctx context.Context, p *provider.Provider[*cockroach.DatabaseIntent]) error {
	instanceID := uuid.NewString()
	log.Info().Str("instance_id", instanceID).Msg("starting migration provider")

	cluster := &cockroachclass.ClusterInstance{}
	if err := p.Resources.Unmarshal(fmt.Sprintf("%s:cluster", providerPkg), cluster); err != nil {
		return fmt.Errorf("unable to read required resource \"cluster\": %w", err)
	}

	// TODO inject file as secret ref and propagate secret ref to server, too.
	if cluster.CaCert != "" {
		if err := os.WriteFile(caCertPath, []byte(cluster.CaCert), 0644); err != nil {
			return fmt.Errorf("failed to write %q: %w", caCertPath, err)
		}

		if err := os.Setenv("PGSSLROOTCERT", caCertPath); err != nil {
			return fmt.Errorf("failed to set PGSSLROOTCERT: %w", err)
		}

	}

	var sb strings.Builder
	if len(p.Intent.Regions) > 0 {
		sb.WriteString(fmt.Sprintf("PRIMARY REGION %q REGIONS ", p.Intent.Regions[0]))
		for i, region := range p.Intent.Regions {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(fmt.Sprintf("%q", region))
		}

		if p.Intent.SurvivalGoal != "" {
			sb.WriteString(fmt.Sprintf(" SURVIVE %s FAILURE", p.Intent.SurvivalGoal))
		}
	}

	exists, err := helpers.EnsureDatabase(ctx, cluster, p.Intent.Name, sb.String())
	if err != nil {
		return fmt.Errorf("unable to create database %q: %w", p.Intent.Name, err)
	}

	instance := &cockroachclass.DatabaseInstance{
		ConnectionUri:  postgres.ConnectionUri(cluster, p.Intent.Name),
		Name:           p.Intent.Name,
		User:           postgres.UserOrDefault(cluster.User),
		Password:       cluster.Password,
		ClusterAddress: cluster.Address,
		ClusterHost:    cluster.Host,
		ClusterPort:    cluster.Port,
		SslMode:        cluster.SslMode,
		EnableTracing:  p.Intent.EnableTracing,
		SurvivalGoal:   p.Intent.SurvivalGoal,
		Regions:        p.Intent.Regions,
	}

	client := fmt.Sprintf("provider:%s", p.Intent.Name)
	db, err := universepg.NewDatabaseFromConnectionUriWithOverrides(ctx, instance, instance.ConnectionUri, nil, client, &universepg.ConfigOverrides{
		MaxConnIdleTime: connIdleTimeout,
	})
	if err != nil {
		return fmt.Errorf("unable to open connection: %w", err)
	}

	defer func() {
		if err := db.Close(); err != nil {
			log.Warn().Err(err).Msg("unable to close database connection")
		}
	}()

	if err := ensureMigrationMetadata(ctx, db); err != nil {
		return fmt.Errorf("unable to ensure migration metadata: %w", err)
	}

	if !exists || !p.Intent.SkipSchemaInitializationIfExists {
		for _, oneSchema := range p.Intent.Schema {
			if oneSchema.Path == "" {
				return fmt.Errorf("migration file has empty path")
			}

			if err := applyMigrationWithLock(ctx, db, instanceID, oneSchema.Path, oneSchema.Contents); err != nil {
				return fmt.Errorf("unable to apply schema %q: %w", oneSchema.Path, err)
			}
		}
	}

	p.EmitResult(instance)
	return nil
}

func ensureMigrationMetadata(ctx context.Context, db *universepg.DB) error {
	schemaExists, err := queryBool(ctx, db,
		`SELECT EXISTS (SELECT 1 FROM information_schema.schemata WHERE schema_name = 'foundation')`)
	if err != nil {
		return fmt.Errorf("checking foundation schema: %w", err)
	}

	if !schemaExists {
		if err := applyWithRetry(ctx, db, `CREATE SCHEMA IF NOT EXISTS foundation`); err != nil {
			return fmt.Errorf("creating foundation schema: %w", err)
		}
	}

	tableExists, err := queryBool(ctx, db,
		`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'foundation' AND table_name = 'migrations')`)
	if err != nil {
		return fmt.Errorf("checking migrations table: %w", err)
	}

	if !tableExists {
		if err := applyWithRetry(ctx, db, `CREATE TABLE IF NOT EXISTS foundation.migrations (
			path STRING PRIMARY KEY,
			checksum STRING NOT NULL,
			migrating UUID NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`); err != nil {
			return fmt.Errorf("creating migrations table: %w", err)
		}
	}

	return nil
}

func queryBool(ctx context.Context, db *universepg.DB, query string) (bool, error) {
	var result bool
	if err := db.QueryRow(ctx, query).Scan(&result); err != nil {
		return false, err
	}
	return result, nil
}

func migrationChecksum(contents []byte) string {
	sum := sha256.Sum256(contents)
	return fmt.Sprintf("%x", sum)
}

type migrationState struct {
	Checksum  string
	Migrating sql.NullString
	CreatedAt time.Time
	Stale     bool
}

func getMigrationState(ctx context.Context, db *universepg.DB, path string) (*migrationState, error) {
	var s migrationState
	err := db.QueryRow(ctx,
		`SELECT checksum, migrating::STRING, created_at, created_at < now() - INTERVAL '10 minutes' AS stale
		 FROM foundation.migrations WHERE path = $1`, path).Scan(&s.Checksum, &s.Migrating, &s.CreatedAt, &s.Stale)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, nil
		}
		return nil, err
	}
	return &s, nil
}

func tryAcquireMigration(ctx context.Context, db *universepg.DB, instanceID, path, checksum string) (bool, error) {
	var acquired bool
	err := backoff.Retry(func() error {
		tag, err := db.Exec(ctx,
			`INSERT INTO foundation.migrations (path, checksum, migrating, created_at)
			 VALUES ($1, $2, $3::UUID, now())
			 ON CONFLICT (path) DO UPDATE SET
			     checksum = excluded.checksum,
			     migrating = excluded.migrating,
			     created_at = now()
			 WHERE
			     (foundation.migrations.migrating IS NULL AND foundation.migrations.checksum <> excluded.checksum)
			     OR
			     (foundation.migrations.migrating IS NOT NULL AND foundation.migrations.created_at < now() - INTERVAL '10 minutes')`,
			path, checksum, instanceID)
		if err != nil {
			if !universepg.ErrorIsRetryable(err) {
				return backoff.Permanent(err)
			}
			return err
		}
		acquired = tag.RowsAffected() == 1
		return nil
	}, helpers.BackOff{
		Interval: 100 * time.Millisecond,
		Deadline: time.Now().Add(15 * time.Second),
		Jitter:   100 * time.Millisecond,
	})
	return acquired, err
}

func completeMigration(ctx context.Context, db *universepg.DB, instanceID, path, checksum string) error {
	return backoff.Retry(func() error {
		tag, err := db.Exec(ctx,
			`UPDATE foundation.migrations SET checksum = $2, migrating = NULL WHERE path = $1 AND migrating = $3::UUID`,
			path, checksum, instanceID)
		if err != nil {
			if !universepg.ErrorIsRetryable(err) {
				return backoff.Permanent(err)
			}
			return err
		}
		if tag.RowsAffected() != 1 {
			return backoff.Permanent(fmt.Errorf("lost migration ownership for %q before completion", path))
		}
		return nil
	}, helpers.BackOff{
		Interval: 100 * time.Millisecond,
		Deadline: time.Now().Add(15 * time.Second),
		Jitter:   100 * time.Millisecond,
	})
}

func applyMigrationWithLock(ctx context.Context, db *universepg.DB, instanceID, path string, contents []byte) error {
	checksum := migrationChecksum(contents)
	sqlText := string(contents)

	for {
		acquired, err := tryAcquireMigration(ctx, db, instanceID, path, checksum)
		if err != nil {
			return err
		}

		if acquired {
			log.Info().Str("path", path).Msg("acquired migration lock")

			if err := applyWithRetry(ctx, db, sqlText); err != nil {
				return err
			}

			return completeMigration(ctx, db, instanceID, path, checksum)
		}

		state, err := getMigrationState(ctx, db, path)
		if err != nil {
			return err
		}

		if state == nil {
			continue
		}

		switch {
		case !state.Migrating.Valid && state.Checksum == checksum:
			log.Info().Str("path", path).Msg("skipping already-applied migration")
			return nil

		case state.Migrating.Valid && state.Migrating.String == instanceID:
			log.Info().Str("path", path).Msg("re-acquired own migration lock")
			if err := applyWithRetry(ctx, db, sqlText); err != nil {
				return err
			}
			return completeMigration(ctx, db, instanceID, path, checksum)

		case !state.Migrating.Valid:
			continue

		case state.Stale:
			log.Warn().Str("path", path).Msg("taking over stale migration lock")
			continue

		default:
			log.Info().Str("path", path).Str("owner", state.Migrating.String).Msg("waiting for active migration")
			if err := sleepWithContext(ctx, migrationPollInterval); err != nil {
				return err
			}
		}
	}
}

func sleepWithContext(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func applyWithRetry(ctx context.Context, db *universepg.DB, sql string) error {
	return backoff.Retry(func() error {
		_, err := db.Exec(ctx, sql)

		if !universepg.ErrorIsRetryable(err) {
			return backoff.Permanent(err)
		}

		return err
	}, helpers.BackOff{
		Interval: 10 * time.Second,
		// Leave more time for migrations to run since schema changes are relatively slow
		Deadline: time.Now().Add(5 * time.Minute),
		Jitter:   5 * time.Second,
	})
}
