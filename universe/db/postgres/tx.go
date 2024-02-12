// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package postgres

import (
	"context"
	"errors"

	"github.com/cenkalti/backoff/v4"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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
		attribute.String("pg.access-mode", string(txoptions.AccessMode))).Attribute(db.traceAttributes()...)
	if attempt > 0 {
		n = n.Attribute(attribute.Int64("db.tx_attempt", attempt))
	}

	return tracing.Collect1(ctx, db.t, n, func(ctx context.Context) (T, error) {
		var empty T

		tx, err := db.base.BeginTx(ctx, txoptions)
		if err != nil {
			return empty, TransactionError{err}
		}

		defer func() { _ = tx.Rollback(ctx) }()

		value, err := f(ctx, tx)
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
