// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package postgres

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
	"namespacelabs.dev/foundation/std/go/core"
)

// pgx does not provide for instrumentation hooks, only logging. So we wrap access to it, retaining the API.
// Alternatively, https://github.com/uptrace/opentelemetry-go-extra/tree/otelsql/v0.1.12/otelsql could be used
// but that requires database/sql, which does not support pg-specific types.

type DB struct {
	pool   *pgxpool.Pool
	tracer trace.Tracer
}

func (db DB) Exec(ctx context.Context, sql string, arguments ...interface{}) (pgconn.CommandTag, error) {
	var tag pgconn.CommandTag
	spanErr := db.withSpan(ctx, "db.Exec", sql, func(ctx context.Context) (err error) {
		tag, err = db.pool.Exec(ctx, sql, arguments...)
		return err
	})
	return tag, spanErr
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

	ctx, span := db.tracer.Start(ctx, name,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(semconv.DBStatementKey.String(sql)))
	err := f(ctx)
	span.End()

	if span.IsRecording() {
		if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, pgx.ErrNoRows) {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
	}

	return err
}

type WireDatabase struct {
	ready core.Check
	otel  trace.TracerProvider
}

func logf(message string, args ...interface{}) {
	fmt.Fprintf(os.Stdout, "%s : %s\n", time.Now().String(), fmt.Sprintf(message, args...))
}

func (w WireDatabase) ProvideDatabase(ctx context.Context, db *Database, username string, password string) (*DB, error) {
	// Config has to be created by ParseConfig
	config, err := pgxpool.ParseConfig(fmt.Sprintf("postgres://%s:%s@%s:%d/%s",
		username,
		password,
		db.HostedAt.Address,
		db.HostedAt.Port,
		db.Name))
	if err != nil {
		return nil, err
	}

	// Only connect when the pool starts to be used.
	config.LazyConnect = true

	conn, err := pgxpool.ConnectConfig(ctx, config)
	if err != nil {
		logf("failed to connect: %v", err)
		return nil, err
	}

	// Asynchronously wait until a database connection is ready.
	w.ready.RegisterFunc(fmt.Sprintf("%s/%s", core.InstantiationPathFromContext(ctx), db.Name), func(ctx context.Context) error {
		return conn.Ping(ctx)
	})

	var tracer trace.Tracer
	if w.otel != nil {
		tracer = otel.Tracer(Package__sfr1nt.PackageName)
	}

	return &DB{conn, tracer}, nil
}

func ProvideWireDatabase(_ context.Context, _ *WireDatabaseArgs, deps ExtensionDeps) (WireDatabase, error) {
	t, err := deps.OpenTelemetry.GetTracerProvider()
	if err != nil {
		return WireDatabase{}, err
	}

	return WireDatabase{deps.ReadinessCheck, t}, nil
}
