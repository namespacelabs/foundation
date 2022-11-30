// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
	"go.opentelemetry.io/otel/trace"
)

func ReturnFromTx[T any](ctx context.Context, db *DB, f func(context.Context, pgx.Tx) (T, error)) (T, error) {
	var empty T

	tx, err := db.base.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return empty, err
	}

	defer tx.Rollback(ctx)

	value, err := f(ctx, tracingTx{base: tx, t: db.t})
	if err != nil {
		return empty, err
	}

	if err := tx.Commit(ctx); err != nil {
		return empty, err
	}

	return value, nil
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
	rows, _ := tx.Query(ctx, sql, args...)
	// This relies on an implementation detail: both pgx.Rows also implement the
	// `pgx.Row` interface, and that a `pgx.Rows` carries an error. Refer to
	// pgx's source code for details.
	return rows.(pgx.Row)
}

func (tx tracingTx) QueryFunc(ctx context.Context, sql string, args []interface{}, scans []interface{}, f func(pgx.QueryFuncRow) error) (pgconn.CommandTag, error) {
	return returnWithSpan(ctx, tx.t, "tx.QueryFunc", sql, func(ctx context.Context) (pgconn.CommandTag, error) {
		return tx.base.QueryFunc(ctx, sql, args, scans, f)
	})
}

func (tx tracingTx) Conn() *pgx.Conn { return tx.base.Conn() }
