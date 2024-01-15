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
	"go.opentelemetry.io/otel/attribute"
	"golang.org/x/exp/slices"
	"namespacelabs.dev/foundation/framework/tracing"
)

const (
	pgSerializationFailure         = "40001"
	pgDeadlockFailure              = "40P01"
	pgUniqueConstraintViolation    = "23505"
	pgExclusionConstraintViolation = "23P01"
	pgAdminShutdown                = "57P01" // PG restarts
)

// https://www.postgresql.org/docs/current/mvcc-serialization-failure-handling.html
var retryableSqlStates = []string{
	pgSerializationFailure,
	pgDeadlockFailure,
	pgUniqueConstraintViolation,
	pgExclusionConstraintViolation,
	pgAdminShutdown,
}

func ReturnFromReadWriteTx[T any](ctx context.Context, db *DB, b backoff.BackOff, f func(context.Context, pgx.Tx) (T, error)) (T, error) {
	var attempt int64
	return backoff.RetryWithData(func() (T, error) {
		value, err := doTxFunc(ctx, db, pgx.TxOptions{IsoLevel: pgx.Serializable}, attempt, f)
		if err == nil {
			return value, nil
		}

		attempt++
		if !ErrorIsRetryable(err) {
			return value, backoff.Permanent(err)
		}

		return value, err
	}, b)
}

func ReturnFromTx[T any](ctx context.Context, db *DB, txoptions pgx.TxOptions, f func(context.Context, pgx.Tx) (T, error)) (T, error) {
	return doTxFunc(ctx, db, pgx.TxOptions{IsoLevel: pgx.Serializable}, 0, f)
}

func doTxFunc[T any](ctx context.Context, db *DB, txoptions pgx.TxOptions, attempt int64, f func(context.Context, pgx.Tx) (T, error)) (T, error) {
	n := tracing.Name("pg.Transaction").Attribute(
		attribute.String("pg.isolation-level", string(txoptions.IsoLevel)),
		attribute.String("pg.access-mode", string(txoptions.AccessMode))).Attribute(db.opts.TraceAttributes()...)
	if attempt > 0 {
		n = n.Attribute(attribute.Int64("db.tx_attempt", attempt))
	}

	return tracing.Collect1(ctx, db.Tracer(), n, func(ctx context.Context) (T, error) {
		var empty T

		tx, err := db.base.BeginTx(ctx, txoptions)
		if err != nil {
			return empty, TransactionError{err}
		}

		defer func() { _ = tx.Rollback(ctx) }()

		value, err := f(ctx, tracingTx{base: tx, opts: db.opts})
		if err != nil {
			return empty, err
		}

		if err := tx.Commit(ctx); err != nil {
			return empty, TransactionError{err}
		}

		return value, nil
	})
}

func ErrorIsRetryable(err error) bool {
	var pgerr *pgconn.PgError
	if !errors.As(err, &pgerr) {
		return false
	}

	return slices.Contains(retryableSqlStates, pgerr.SQLState())
}

type TransactionError struct {
	InternalErr error
}

func (p TransactionError) Error() string { return p.InternalErr.Error() }
func (p TransactionError) Unwrap() error { return p.InternalErr }

type tracingTx struct {
	base pgx.Tx
	opts commonOpts
}

func (tx tracingTx) Begin(ctx context.Context) (pgx.Tx, error) {
	return returnWithSpan(ctx, tx.opts, "tx.Begin", "", func(ctx context.Context) (pgx.Tx, error) {
		newtx, err := tx.base.Begin(ctx)
		return tracingTx{newtx, tx.opts}, err
	})
}

func (tx tracingTx) BeginFunc(ctx context.Context, f func(pgx.Tx) error) error {
	return withSpan(ctx, tx.opts, "tx.BeginFunc", "", func(ctx context.Context) error {
		return tx.base.BeginFunc(ctx, func(newtx pgx.Tx) error {
			return f(tracingTx{base: newtx, opts: tx.opts})
		})
	})
}

func (tx tracingTx) Commit(ctx context.Context) error {
	return withSpan(ctx, tx.opts, "tx.Commit", "", func(ctx context.Context) error {
		return tx.base.Commit(ctx)
	})
}

func (tx tracingTx) Rollback(ctx context.Context) error {
	return withSpan(ctx, tx.opts, "tx.Rollback", "", func(ctx context.Context) error {
		return tx.base.Commit(ctx)
	})
}

func (tx tracingTx) CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error) {
	return returnWithSpan(ctx, tx.opts, "tx.CopyFrom", "", func(ctx context.Context) (int64, error) {
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
	return returnWithSpan(ctx, tx.opts, "tx.Prepare", fmt.Sprintf("%s = %s", name, sql), func(ctx context.Context) (*pgconn.StatementDescription, error) {
		return tx.base.Prepare(ctx, name, sql)
	})
}

func (tx tracingTx) Exec(ctx context.Context, sql string, arguments ...interface{}) (pgconn.CommandTag, error) {
	return returnWithSpan(ctx, tx.opts, "tx.Exec", sql, func(ctx context.Context) (pgconn.CommandTag, error) {
		return tx.base.Exec(ctx, sql, arguments...)
	})
}

func (tx tracingTx) Query(ctx context.Context, sql string, arguments ...interface{}) (pgx.Rows, error) {
	return returnWithSpan(ctx, tx.opts, "tx.Query", sql, func(ctx context.Context) (pgx.Rows, error) {
		return tx.base.Query(ctx, sql, arguments...)
	})
}

func (tx tracingTx) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	return queryRow(ctx, tx.opts, tx.base, "tx.QueryRow", sql, args...)
}

func (tx tracingTx) QueryFunc(ctx context.Context, sql string, args []interface{}, scans []interface{}, f func(pgx.QueryFuncRow) error) (pgconn.CommandTag, error) {
	return returnWithSpan(ctx, tx.opts, "tx.QueryFunc", sql, func(ctx context.Context) (pgconn.CommandTag, error) {
		return tx.base.QueryFunc(ctx, sql, args, scans, f)
	})
}

func (tx tracingTx) Conn() *pgx.Conn { return tx.base.Conn() }
