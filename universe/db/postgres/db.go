// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package postgres

import (
	"context"
	"errors"
	"io"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
	statuscodes "google.golang.org/grpc/codes"
	"namespacelabs.dev/foundation/std/go/rpcerrors"
)

// pgx does not provide for instrumentation hooks, only logging. So we wrap access to it, retaining the API.
// Alternatively, https://github.com/uptrace/opentelemetry-go-extra/tree/otelsql/v0.1.12/otelsql could be used
// but that requires database/sql, which does not support pg-specific types.

type DB struct {
	pool   *pgxpool.Pool
	tracer trace.Tracer
}

func NewDB(conn *pgxpool.Pool, tracer trace.Tracer) *DB {
	return &DB{pool: conn, tracer: tracer}
}

func (db DB) Exec(ctx context.Context, sql string, arguments ...interface{}) (pgconn.CommandTag, error) {
	var tag pgconn.CommandTag
	spanErr := db.withSpan(ctx, "db.Exec", sql, func(ctx context.Context) (err error) {
		tag, err = db.pool.Exec(ctx, sql, arguments...)
		return err
	})
	return tag, spanErr
}

func (db DB) BeginTxFunc(ctx context.Context, txOptions pgx.TxOptions, callback func(pgx.Tx) error) error {
	return db.withSpan(ctx, "db.BeginTxFunc", "", func(ctx context.Context) error {
		// XXX wrap transaction for span logging.
		return db.pool.BeginTxFunc(ctx, txOptions, callback)
	})
}

func (db DB) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	var rows pgx.Rows
	spanErr := db.withSpan(ctx, "db.Query", sql, func(ctx context.Context) (err error) {
		rows, err = db.pool.Query(ctx, sql, args...)
		return err
	})
	return rows, spanErr
}

func (db DB) QueryFunc(ctx context.Context, sql string, args []interface{}, scans []interface{}, f func(pgx.QueryFuncRow) error) (pgconn.CommandTag, error) {
	var tag pgconn.CommandTag
	spanErr := db.withSpan(ctx, "db.QueryFunc", sql, func(ctx context.Context) (err error) {
		tag, err = db.pool.QueryFunc(ctx, sql, args, scans, f)
		return err
	})
	return tag, spanErr
}

func (db DB) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	var row pgx.Row
	_ = db.withSpan(ctx, "db.QueryRow", sql, func(ctx context.Context) error {
		row = db.pool.QueryRow(ctx, sql, args...)
		return nil
	})
	return row
}

func (db DB) withSpan(ctx context.Context, name, sql string, f func(context.Context) error) error {
	if db.tracer == nil {
		return f(ctx)
	}

	options := []trace.SpanStartOption{trace.WithSpanKind(trace.SpanKindClient)}

	if sql != "" {
		options = append(options, trace.WithAttributes(semconv.DBStatementKey.String(sql)))
	}

	ctx, span := db.tracer.Start(ctx, name, options...)
	defer span.End()

	err := checkErr(f(ctx))
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, pgx.ErrNoRows) {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}

	return err
}

func checkErr(err error) error {
	if errors.Is(err, io.ErrUnexpectedEOF) {
		return rpcerrors.Wrap(statuscodes.Unavailable, err)
	}

	return err
}
