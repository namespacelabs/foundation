// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package postgres

import (
	"context"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgproto3/v2"
	"github.com/jackc/pgx/v4"
)

type hasQuery interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func query(ctx context.Context, opts commonOpts, q hasQuery, name, sql string, args ...any) (pgx.Rows, error) {
	rows, err := returnWithSpan(ctx, opts, name, sql, func(ctx context.Context) (pgx.Rows, error) {
		return q.Query(ctx, sql, args...)
	})
	if err != nil {
		return nil, err
	}

	return tracedRows{ctx, opts, sql, rows}, nil
}

type tracedRows struct {
	ctx  context.Context
	opts commonOpts
	sql  string
	rows pgx.Rows
}

func (r tracedRows) Close() {
	// Ensure that any error observed while reading rows is reported to a span.
	_ = withSpan(r.ctx, r.opts, "db.rows.Close", r.sql, func(context.Context) error {
		r.rows.Close()

		return r.rows.Err()
	})
}

func (r tracedRows) Err() error {
	return r.rows.Err()
}

func (r tracedRows) CommandTag() pgconn.CommandTag {
	return r.rows.CommandTag()
}

func (r tracedRows) FieldDescriptions() []pgproto3.FieldDescription {
	return r.rows.FieldDescriptions()
}

func (r tracedRows) Next() bool {
	return r.rows.Next()
}

func (r tracedRows) Scan(dest ...interface{}) error {
	return r.rows.Scan(dest...)
}

func (r tracedRows) Values() ([]interface{}, error) {
	return r.rows.Values()
}

func (r tracedRows) RawValues() [][]byte {
	return r.rows.RawValues()
}
