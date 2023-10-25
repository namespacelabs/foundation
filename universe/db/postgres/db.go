// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/trace"
	postgrespb "namespacelabs.dev/foundation/library/database/postgres"
)

// pgx does not provide for instrumentation hooks, only logging. So we wrap access to it, retaining the API.
// Alternatively, https://github.com/uptrace/opentelemetry-go-extra/tree/otelsql/v0.1.12/otelsql could be used
// but that requires database/sql, which does not support pg-specific types.

type DB struct {
	base *pgxpool.Pool
	opts commonOpts
}

type commonOpts struct {
	t    trace.Tracer
	errw func(context.Context, error) error
}

var (
	metrics = []struct {
		Key   string
		Value func(*pgxpool.Stat) float64
	}{
		{"acquire_count", func(s *pgxpool.Stat) float64 { return float64(s.AcquireCount()) }},
		{"acquired_conns", func(s *pgxpool.Stat) float64 { return float64(s.AcquiredConns()) }},
		{"canceled_acquire_count", func(s *pgxpool.Stat) float64 { return float64(s.CanceledAcquireCount()) }},
		{"acquire_duration_ms", func(s *pgxpool.Stat) float64 { return float64(s.AcquireDuration().Milliseconds()) }},
		{"constructing_conns", func(s *pgxpool.Stat) float64 { return float64(s.ConstructingConns()) }},
		{"empty_acquire_count", func(s *pgxpool.Stat) float64 { return float64(s.EmptyAcquireCount()) }},
		{"idle_conns", func(s *pgxpool.Stat) float64 { return float64(s.IdleConns()) }},
		{"max_conns", func(s *pgxpool.Stat) float64 { return float64(s.MaxConns()) }},
		{"total_conns", func(s *pgxpool.Stat) float64 { return float64(s.TotalConns()) }},
	}
	cols []*prometheus.GaugeVec
)

func init() {
	cols = make([]*prometheus.GaugeVec, len(metrics))
	for k, def := range metrics {
		cols[k] = prometheus.NewGaugeVec(prometheus.GaugeOpts{Subsystem: "pgx_pool", Name: def.Key}, []string{"address", "database"})
		prometheus.MustRegister(cols[k])
	}
}

type NewDBOptions struct {
	Tracer       trace.Tracer
	ErrorWrapper func(context.Context, error) error
}

func NewDB(instance *postgrespb.DatabaseInstance, conn *pgxpool.Pool, o NewDBOptions) *DB {
	db := &DB{base: conn, opts: commonOpts{o.Tracer, o.ErrorWrapper}}
	if db.opts.errw == nil {
		db.opts.errw = func(_ context.Context, err error) error { return err }
	}

	if instance != nil {
		go func() {
			// Connections never go away, so we never stop updating metrics.
			time.Sleep(5 * time.Second)

			stats := conn.Stat()

			for k, def := range metrics {
				cols[k].WithLabelValues(instance.ClusterAddress, instance.Name).Set(def.Value(stats))
			}
		}()
	}

	return db
}

func (db DB) Exec(ctx context.Context, sql string, arguments ...interface{}) (pgconn.CommandTag, error) {
	return returnWithSpan(ctx, db.opts, "db.Exec", sql, func(ctx context.Context) (pgconn.CommandTag, error) {
		return db.base.Exec(ctx, sql, arguments...)
	})
}

func (db DB) BeginTxFunc(ctx context.Context, txOptions pgx.TxOptions, callback func(pgx.Tx) error) error {
	return withSpan(ctx, db.opts, "db.BeginTxFunc", "", func(ctx context.Context) error {
		return db.base.BeginTxFunc(ctx, txOptions, func(newtx pgx.Tx) error {
			return callback(tracingTx{base: newtx, opts: db.opts})
		})
	})
}

func (db DB) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	return query(ctx, db.opts, db.base, "db.Query", sql, args...)
}

func (db DB) QueryFunc(ctx context.Context, sql string, args []interface{}, scans []interface{}, f func(pgx.QueryFuncRow) error) (pgconn.CommandTag, error) {
	return returnWithSpan(ctx, db.opts, "db.QueryFunc", sql, func(ctx context.Context) (pgconn.CommandTag, error) {
		return db.base.QueryFunc(ctx, sql, args, scans, f)
	})
}

func (db DB) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	return queryRow(ctx, db.opts, db.base, "db.QueryRow", sql, args...)
}

func (db DB) SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults {
	// Unfortunately can't introspect pgx.Batch
	sql := fmt.Sprintf("batch(%d)", b.Len())
	res, _ := returnWithSpan(ctx, db.opts, "db.SendBatch", sql, func(ctx context.Context) (pgx.BatchResults, error) {
		return db.base.SendBatch(ctx, b), nil
	})
	return res
}

func (db DB) Tracer() trace.Tracer { return db.opts.t }
