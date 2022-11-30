// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package postgres

import (
	"context"

	"github.com/jackc/pgx/v4"
	"go.opentelemetry.io/otel/trace"
)

type hasQuery interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func queryRow(ctx context.Context, t trace.Tracer, q hasQuery, name, sql string, args ...any) pgx.Row {
	return deferredRow{ctx, t, q, name, sql, args}
}

type deferredRow struct {
	ctx  context.Context
	t    trace.Tracer
	q    hasQuery
	name string
	sql  string
	args []any
}

func (d deferredRow) Scan(target ...any) error {
	return withSpan(d.ctx, d.t, d.name, d.sql, func(ctx context.Context) error {
		rows, err := d.q.Query(ctx, d.sql, d.args...)
		if err != nil {
			return err
		}

		defer rows.Close()

		for rows.Next() {
			return rows.Scan(target...)
		}

		return pgx.ErrNoRows
	})
}
