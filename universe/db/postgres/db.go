// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

// Copyright 2022, 2025 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

type DB struct {
	base   *pgxpool.Pool
	opts   commonOpts
	t      trace.Tracer
	cancel func()
}

func (db DB) GetClusterAddress() string { return db.opts.clusterAddr }
func (db DB) GetName() string           { return db.opts.databaseName }
func (db DB) GetEnableTracing() bool    { return db.opts.enableTracing }

type DBInstance interface {
	GetClusterAddress() string
	GetName() string
	GetEnableTracing() bool
}

type commonOpts struct {
	clusterAddr   string
	databaseName  string
	enableTracing bool
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
		cols[k] = prometheus.NewGaugeVec(prometheus.GaugeOpts{Subsystem: "pgx_pool", Name: def.Key}, []string{"address", "database", "client"})
		prometheus.MustRegister(cols[k])
	}
}

func newDatabase(instance DBInstance, conn *pgxpool.Pool, tracer trace.Tracer, client string) *DB {
	db := &DB{base: conn, t: tracer}

	if instance != nil {
		db.opts = commonOpts{
			clusterAddr:   instance.GetClusterAddress(),
			databaseName:  instance.GetName(),
			enableTracing: instance.GetEnableTracing(),
		}

	}

	if cfg := conn.Config().ConnConfig; cfg != nil {
		if db.opts.clusterAddr == "" {
			db.opts.clusterAddr = fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
		}

		if db.opts.databaseName == "" {
			db.opts.databaseName = cfg.Database
		}
	}

	if db.opts.clusterAddr != "" && db.opts.databaseName != "" {
		ctx, cancel := context.WithCancel(context.Background())

		go func() {
			t := time.NewTicker(5 * time.Second)
			defer t.Stop()

			for {
				select {
				case <-ctx.Done():
					return

				case <-t.C:
					stats := conn.Stat()

					for k, def := range metrics {
						cols[k].WithLabelValues(db.opts.clusterAddr, db.opts.databaseName, client).Set(def.Value(stats))
					}
				}
			}
		}()

		db.cancel = cancel
	}

	return db
}

func (db DB) traceAttributes() []attribute.KeyValue {
	var keyvalues []attribute.KeyValue
	if db.opts.clusterAddr != "" {
		keyvalues = append(keyvalues, attribute.String("db.host", db.opts.clusterAddr))
	}

	if db.opts.databaseName != "" {
		keyvalues = append(keyvalues, semconv.DBNamespace(db.opts.databaseName))
	}

	return keyvalues
}

func (db DB) Close() error {
	if db.cancel != nil {
		db.cancel()
	}

	return nil
}

func (db DB) Exec(ctx context.Context, sql string, arguments ...interface{}) (pgconn.CommandTag, error) {
	return db.base.Exec(ctx, sql, arguments...)
}

func (db DB) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	return db.base.Query(ctx, sql, args...)
}

func (db DB) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	return db.base.QueryRow(ctx, sql, args...)
}

func (db DB) SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults {
	return db.base.SendBatch(ctx, b)
}

func (db DB) PgxPool() *pgxpool.Pool {
	return db.base
}
