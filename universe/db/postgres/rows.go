// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package postgres

import (
	"context"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgproto3/v2"
	"github.com/jackc/pgx/v4"
	"go.opentelemetry.io/otel/trace"
)

type hasQuery interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func query(ctx context.Context, opts commonOpts, q hasQuery, name, sql string, args ...any) (pgx.Rows, error) {
	// span may be nil.
	ctx, span := createSpan(ctx, opts, name, sql)

	rows, err := q.Query(ctx, sql, args...)
	if err != nil {
		if span != nil {
			defer span.End()
		}

		return nil, processError(ctx, span, opts, err)
	}

	if span == nil {
		return rows, nil
	}

	span.AddEvent("QueryReturned")

	return tracedRows{span, rows}, nil
}

type tracedRows struct {
	span trace.Span
	rows pgx.Rows
}

var _ pgx.Rows = tracedRows{}

func (r tracedRows) Close() {
	defer r.span.End()

	r.rows.Close()
	// Ensure that any error observed while reading rows is reported to a span.
	if err := r.rows.Err(); err != nil {
		recordErr(r.span, err)
	}
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
