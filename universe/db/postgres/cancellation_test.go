// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package postgres

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"gotest.tools/assert"
)

func TestCancellationPreservesConnections(t *testing.T) {
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
	uri := fmt.Sprintf("postgres://test:test@%s:%s/test?sslmode=disable", host, port.Port())
	observer, err := pgx.Connect(ctx, uri)
	assert.NilError(t, err)
	t.Cleanup(func() { assert.NilError(t, observer.Close(context.Background())) })

	open := func(t *testing.T, grace time.Duration) *DB {
		t.Helper()
		db, err := NewDatabaseFromConnectionUriWithOverrides(ctx, nil, uri, nil, "cancellation-test", &ConfigOverrides{
			MaxConns:          1,
			CancellationGrace: grace,
		})
		assert.NilError(t, err)
		t.Cleanup(func() {
			db.PgxPool().Close()
			assert.NilError(t, db.Close())
		})
		return db
	}
	pid := func(t *testing.T, db *DB) int32 {
		t.Helper()
		var id int32
		assert.NilError(t, db.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&id))
		return id
	}

	for _, tc := range []struct {
		name  string
		grace time.Duration
		sleep string
		reuse bool
	}{
		{name: "immediate cancellation closes connection", sleep: "10"},
		{name: "query completes during grace", grace: 2 * time.Second, sleep: "0.3", reuse: true},
		{name: "grace expiry closes connection", grace: 100 * time.Millisecond, sleep: "10"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := open(t, tc.grace)
			before := pid(t, db)
			queryCtx, cancel := context.WithCancel(ctx)
			defer cancel()
			err := cancelSleepingQuery(t, ctx, observer, before, cancel, func() error {
				_, err := db.Exec(queryCtx, "SELECT pg_sleep("+tc.sleep+")")
				return err
			})
			if tc.reuse {
				assert.NilError(t, err)
				assert.Equal(t, pid(t, db), before)
			} else {
				assert.Assert(t, errors.Is(err, context.Canceled), "expected cancellation, got %v", err)
				assert.Assert(t, pid(t, db) != before)
			}
		})
	}

	t.Run("transactions roll back canceled writes and reuse connection", func(t *testing.T) {
		for _, mode := range []string{"commit", "canceled callback", "callback error", "canceled statement"} {
			t.Run(mode, func(t *testing.T) {
				db := open(t, 2*time.Second)
				_, err := db.Exec(ctx, "CREATE TEMP TABLE cancellation_writes (id integer)")
				assert.NilError(t, err)
				before := pid(t, db)
				txCtx, cancel := context.WithCancel(ctx)
				defer cancel()
				run := func() error {
					_, err := ReturnFromTx(txCtx, db, TxOptions{}, func(ctx context.Context, tx pgx.Tx) (int, error) {
						if _, err := tx.Exec(ctx, "INSERT INTO cancellation_writes VALUES (1)"); err != nil {
							return 0, err
						}
						if mode == "canceled statement" {
							_, err := tx.Exec(ctx, "SELECT pg_sleep(0.3)")
							return 1, err
						}
						if mode != "commit" {
							cancel()
						}
						if mode == "callback error" {
							return 0, ctx.Err()
						}
						return 1, nil
					})
					return err
				}
				if mode == "canceled statement" {
					err = cancelSleepingQuery(t, ctx, observer, before, cancel, run)
				} else {
					err = run()
				}
				if mode == "commit" {
					assert.NilError(t, err)
				} else {
					assert.Assert(t, errors.Is(err, context.Canceled), "expected cancellation, got %v", err)
				}
				assert.Equal(t, pid(t, db), before)
				var count int
				assert.NilError(t, db.QueryRow(ctx, "SELECT count(*) FROM cancellation_writes").Scan(&count))
				expected := 0
				if mode == "commit" {
					expected = 1
				}
				assert.Equal(t, count, expected, "only the uncanceled transaction should commit")
			})
		}
	})
}

func cancelSleepingQuery(t *testing.T, ctx context.Context, observer *pgx.Conn, pid int32, cancel context.CancelFunc, run func() error) error {
	t.Helper()
	result := make(chan error, 1)
	go func() { result <- run() }()
	// Observe the query executing before canceling; a canceled pool acquisition cannot prove socket reuse.
	deadline := time.Now().Add(5 * time.Second)
	for {
		var sleeping bool
		assert.NilError(t, observer.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM pg_stat_activity WHERE pid = $1 AND wait_event = 'PgSleep')", pid).Scan(&sleeping))
		if sleeping {
			break
		}
		assert.Assert(t, time.Now().Before(deadline), "query did not enter pg_sleep")
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	select {
	case err := <-result:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("query did not finish within cancellation grace")
		return nil
	}
}
