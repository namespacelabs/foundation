// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
)

type hasQueryRow interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func queryRow(ctx context.Context, opts commonOpts, q hasQueryRow, name, sql string, args ...any) pgx.Row {
	return deferredRow{ctx, opts, q, name, sql, args}
}

type deferredRow struct {
	ctx  context.Context
	opts commonOpts
	q    hasQueryRow
	name string
	sql  string
	args []any
}

func (d deferredRow) Scan(target ...any) error {
	return withSpan(d.ctx, d.opts, d.name, d.sql, func(ctx context.Context) error {
		return d.q.QueryRow(ctx, d.sql, d.args...).Scan(target...)
	})
}
