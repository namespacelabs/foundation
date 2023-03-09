// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/cenkalti/backoff/v4"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/codes"
	"namespacelabs.dev/foundation/framework/rpcerrors"
)

const (
	maxRetries = 5

	pgSerializationFailure      = "40001"
	pgUniqueConstraintViolation = "23505"
)

func ReturnFromReadWriteTx[T any](ctx context.Context, db *DB, b backoff.BackOff, f func(context.Context, pgx.Tx) (T, error)) (T, error) {
	var result T

	i := 0
	err := backoff.Retry(func() error {
		if i == maxRetries {
			return backoff.Permanent(rpcerrors.Errorf(codes.Internal, "max retries exceeded"))
		}

		i++

		value, err := beginRWTxFunc(ctx, db, f)
		if err == nil {
			result = value
			return nil
		}

		if !errorRetryable(err) {
			return backoff.Permanent(err)
		}

		return err
	}, b)

	return result, err
}

func beginRWTxFunc[T any](ctx context.Context, db *DB, f func(context.Context, pgx.Tx) (T, error)) (T, error) {
	var empty T

	tx, err := db.base.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return empty, err
	}

	defer func() { _ = tx.Rollback(ctx) }()

	value, err := f(ctx, tracingTx{base: tx, t: db.t})
	if err != nil {
		return empty, err
	}

	if err := tx.Commit(ctx); err != nil {
		return empty, err
	}

	return value, nil
}

func errorRetryable(err error) bool {
	var pgerr *pgconn.PgError
	if !errors.As(err, &pgerr) {
		return false
	}

	// We need to check unique constraint here because some versions of postgres have an error where
	// unique constraint violations are raised instead of serialization errors.
	// (e.g. https://www.postgresql.org/message-id/flat/CAGPCyEZG76zjv7S31v_xPeLNRuzj-m%3DY2GOY7PEzu7vhB%3DyQog%40mail.gmail.com)
	return pgerr.SQLState() == pgSerializationFailure || pgerr.SQLState() == pgUniqueConstraintViolation
}

type tracingTx struct {
	base pgx.Tx
	t    trace.Tracer
}

func (tx tracingTx) Begin(ctx context.Context) (pgx.Tx, error) {
	return returnWithSpan(ctx, tx.t, "tx.Begin", "", func(ctx context.Context) (pgx.Tx, error) {
		newtx, err := tx.base.Begin(ctx)
		return tracingTx{newtx, tx.t}, err
	})
}

func (tx tracingTx) BeginFunc(ctx context.Context, f func(pgx.Tx) error) error {
	return withSpan(ctx, tx.t, "tx.BeginFunc", "", func(ctx context.Context) error {
		return tx.base.BeginFunc(ctx, func(newtx pgx.Tx) error {
			return f(tracingTx{base: newtx, t: tx.t})
		})
	})
}

func (tx tracingTx) Commit(ctx context.Context) error {
	return withSpan(ctx, tx.t, "tx.Commit", "", func(ctx context.Context) error {
		return tx.base.Commit(ctx)
	})
}

func (tx tracingTx) Rollback(ctx context.Context) error {
	return withSpan(ctx, tx.t, "tx.Rollback", "", func(ctx context.Context) error {
		return tx.base.Commit(ctx)
	})
}

func (tx tracingTx) CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error) {
	return returnWithSpan(ctx, tx.t, "tx.CopyFrom", "", func(ctx context.Context) (int64, error) {
		return tx.base.CopyFrom(ctx, tableName, columnNames, rowSrc)
	})
}

func (tx tracingTx) SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults {
	return tx.base.SendBatch(ctx, b)
}

func (tx tracingTx) LargeObjects() pgx.LargeObjects {
	return tx.base.LargeObjects()
}

func (tx tracingTx) Prepare(ctx context.Context, name, sql string) (*pgconn.StatementDescription, error) {
	return returnWithSpan(ctx, tx.t, "tx.Prepare", fmt.Sprintf("%s = %s", name, sql), func(ctx context.Context) (*pgconn.StatementDescription, error) {
		return tx.base.Prepare(ctx, name, sql)
	})
}

func (tx tracingTx) Exec(ctx context.Context, sql string, arguments ...interface{}) (pgconn.CommandTag, error) {
	return returnWithSpan(ctx, tx.t, "tx.Exec", sql, func(ctx context.Context) (pgconn.CommandTag, error) {
		return tx.base.Exec(ctx, sql, arguments...)
	})
}

func (tx tracingTx) Query(ctx context.Context, sql string, arguments ...interface{}) (pgx.Rows, error) {
	return returnWithSpan(ctx, tx.t, "tx.Query", sql, func(ctx context.Context) (pgx.Rows, error) {
		return tx.base.Query(ctx, sql, arguments...)
	})
}

func (tx tracingTx) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	return queryRow(ctx, tx.t, tx.base, "tx.QueryRow", sql, args...)
}

func (tx tracingTx) QueryFunc(ctx context.Context, sql string, args []interface{}, scans []interface{}, f func(pgx.QueryFuncRow) error) (pgconn.CommandTag, error) {
	return returnWithSpan(ctx, tx.t, "tx.QueryFunc", sql, func(ctx context.Context) (pgconn.CommandTag, error) {
		return tx.base.QueryFunc(ctx, sql, args, scans, f)
	})
}

func (tx tracingTx) Conn() *pgx.Conn { return tx.base.Conn() }
