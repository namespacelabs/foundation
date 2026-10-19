// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package helpers_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"gotest.tools/assert"
	"namespacelabs.dev/foundation/library/oss/postgres"
	"namespacelabs.dev/foundation/library/oss/postgres/prepare/database/helpers"
	"namespacelabs.dev/foundation/schema"
	universepg "namespacelabs.dev/foundation/universe/db/postgres"
)

func TestCreateMissingIndexes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	testcontainers.SkipIfProviderIsNotHealthy(t)
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "postgres:17",
			ExposedPorts: []string{"5432/tcp"},
			Env: map[string]string{
				"POSTGRES_USER":     "test",
				"POSTGRES_PASSWORD": "test",
				"POSTGRES_DB":       "test",
			},
			WaitingFor: wait.ForAll(wait.ForListeningPort("5432/tcp"), wait.ForLog("database system is ready to accept connections").WithOccurrence(2)),
		},
		Started: true,
	})
	assert.NilError(t, err)
	t.Cleanup(func() { assert.NilError(t, container.Terminate(context.Background())) })
	host, err := container.Host(ctx)
	assert.NilError(t, err)
	port, err := container.MappedPort(ctx, "5432/tcp")
	assert.NilError(t, err)
	db, err := universepg.NewDatabaseFromConnectionUri(ctx, nil, fmt.Sprintf("postgres://test:test@%s:%s/test?sslmode=disable", host, port.Port()), nil, "index-policy-test")
	assert.NilError(t, err)
	t.Cleanup(func() { assert.NilError(t, db.Close()) })

	_, err = db.Exec(ctx, "CREATE TABLE items(tenant text, id bigint)")
	assert.NilError(t, err)
	for _, tc := range []struct {
		helper string
		unique bool
		create string
	}{
		{helper: "fn_ensure_index", create: "CREATE INDEX CONCURRENTLY"},
		{helper: "fn_ensure_unique_index", unique: true, create: "CREATE UNIQUE INDEX CONCURRENTLY"},
	} {
		t.Run(tc.helper, func(t *testing.T) {
			intent := &postgres.DatabaseIntent{
				ProvisionHelperFunctions: true,
				Schema: []*schema.FileContents{
					{Path: "index.sql", Contents: []byte(fmt.Sprintf("SELECT %s('ItemsTenant', 'items', '(tenant, id DESC)')", tc.helper))},
				},
			}
			assert.ErrorContains(t, helpers.ApplyWithHelpers(ctx, intent, db), "Missing index")
			intent.CreateMissingIndexes = true
			assert.NilError(t, helpers.ApplyWithHelpers(ctx, intent, db))
			var unique bool
			assert.NilError(t, db.QueryRow(ctx, "SELECT indisunique FROM pg_index WHERE indexrelid = 'itemstenant'::regclass").Scan(&unique))
			assert.Equal(t, unique, tc.unique)
			assert.NilError(t, helpers.ApplyWithHelpers(ctx, intent, db))

			intent.CreateMissingIndexes = false
			assert.NilError(t, helpers.ApplyWithHelpers(ctx, intent, db))
			_, err = db.Exec(ctx, "DROP INDEX itemstenant")
			assert.NilError(t, err)
			intent.Schema = append(intent.Schema, &schema.FileContents{
				Path: "after.sql", Contents: []byte("CREATE TABLE after_index(id bigint)"),
			})
			err = helpers.ApplyWithHelpers(ctx, intent, db)
			var pgerr *pgconn.PgError
			assert.Assert(t, errors.As(err, &pgerr), "expected a PostgreSQL error, got %v", err)
			assert.Equal(t, pgerr.Code, "P0001")
			assert.ErrorContains(t, err, "ItemsTenant")
			assert.ErrorContains(t, err, "items")
			var exists bool
			assert.NilError(t, db.QueryRow(ctx, "SELECT to_regclass('itemstenant') IS NOT NULL OR to_regclass('after_index') IS NOT NULL").Scan(&exists))
			assert.Assert(t, !exists, "the failed apply must create neither the index nor the later table")

			_, err = db.Exec(ctx, tc.create+" ItemsTenant ON items(tenant, id DESC)")
			assert.NilError(t, err)
			assert.NilError(t, helpers.ApplyWithHelpers(ctx, intent, db))
			assert.NilError(t, db.QueryRow(ctx, "SELECT to_regclass('after_index') IS NOT NULL").Scan(&exists))
			assert.Assert(t, exists)
			_, err = db.Exec(ctx, "DROP INDEX itemstenant; DROP TABLE after_index")
			assert.NilError(t, err)
		})
	}
}
