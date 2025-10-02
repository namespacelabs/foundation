// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package postgres

import (
	"context"
	"errors"
	"strings"

	"github.com/cenkalti/backoff/v4"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/exp/slices"
	"namespacelabs.dev/foundation/framework/tracing"
)

// https://www.postgresql.org/docs/current/mvcc-serialization-failure-handling.html
var retryableSqlStates = []string{
	pgerrcode.SerializationFailure,
	pgerrcode.DeadlockDetected,
	pgerrcode.UniqueViolation,
	pgerrcode.ExclusionViolation,
	pgerrcode.AdminShutdown,
}

type TxOptions struct {
	pgx.TxOptions

	EnableTracing bool
}

func ReturnFromReadWriteTx[T any](ctx context.Context, db *DB, b backoff.BackOff, f func(context.Context, pgx.Tx) (T, error)) (T, error) {
	var attempt int64
	return backoff.RetryWithData(func() (T, error) {
		value, err := doTxFunc(ctx, db, TxOptions{TxOptions: pgx.TxOptions{IsoLevel: pgx.Serializable}}, attempt, f)
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

func ReturnFromTx[T any](ctx context.Context, db *DB, txoptions TxOptions, f func(context.Context, pgx.Tx) (T, error)) (T, error) {
	return doTxFunc(ctx, db, txoptions, 0, f)
}

func doTxFunc[T any](ctx context.Context, db *DB, txoptions TxOptions, attempt int64, f func(context.Context, pgx.Tx) (T, error)) (T, error) {
	do := func(ctx context.Context) (T, error) {
		var empty T

		tx, err := db.base.BeginTx(ctx, txoptions.TxOptions)
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
	}

	if !txoptions.EnableTracing {
		// Even if we don't trace the whole tx we want to retain error hints as span attributes.
		return traceHints(ctx, do)
	}

	n := tracing.Name("pg.Transaction").Attribute(
		attribute.String("pg.isolation-level", string(txoptions.IsoLevel)),
		attribute.String("pg.access-mode", string(txoptions.AccessMode))).Attribute(db.traceAttributes()...)
	if attempt > 0 {
		n = n.Attribute(attribute.Int64("db.tx_attempt", attempt))
	}

	return tracing.Collect1(ctx, db.t, n, func(ctx context.Context) (T, error) {
		return traceHints(ctx, do)
	})
}

func traceHints[T any](ctx context.Context, f func(ctx context.Context) (T, error)) (T, error) {
	res, err := f(ctx)
	if err != nil {
		var pgerr *pgconn.PgError
		if errors.As(err, &pgerr) {
			trace.SpanFromContext(ctx).SetAttributes(
				tracing.StringWithCap("pg.error.hint", pgerr.Hint, 256),
				tracing.StringWithCap("pg.error.detail", pgerr.Detail, 256),
			)
		}

		var empty T
		return empty, err
	}

	return res, nil
}

// Returns the error code (to be compared to pgerrcode.* constants).
func PgErrCode(err error) string {
	var pgerr *pgconn.PgError
	if !errors.As(err, &pgerr) {
		return ""
	}
	return pgerr.Code
}

func ErrorIsRetryable(err error) bool {
	var pgerr *pgconn.PgError
	if !errors.As(err, &pgerr) {
		return false
	}

	if pgerr.SQLState() == pgerrcode.InternalError {
		return strings.Contains(pgerr.Message, "tuple concurrently updated")
	}

	return slices.Contains(retryableSqlStates, pgerr.SQLState())
}

type TransactionError struct {
	InternalErr error
}

func (p TransactionError) Error() string { return p.InternalErr.Error() }
func (p TransactionError) Unwrap() error { return p.InternalErr }
