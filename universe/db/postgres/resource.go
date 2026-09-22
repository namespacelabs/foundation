// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package postgres

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"time"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	semconv "go.opentelemetry.io/otel/semconv/v1.41.0"
	"go.opentelemetry.io/otel/trace"
	"namespacelabs.dev/foundation/framework/resources"
	postgrespb "namespacelabs.dev/foundation/library/database/postgres"
)

// Connect to a Postgres Database resource.
func ConnectToResource(ctx context.Context, res *resources.Parsed, resourceRef string, tp trace.TracerProvider, client string, overrides *ConfigOverrides) (*DB, error) {
	db := &postgrespb.DatabaseInstance{}
	if err := res.Unmarshal(resourceRef, db); err != nil {
		return nil, err
	}

	return newDatabaseFromConnectionURI(ctx, db, db.ConnectionUri, db.CaCert, tp, client, overrides)
}

func ConnectToReplicaResource(ctx context.Context, res *resources.Parsed, resourceRef string, tp trace.TracerProvider, client string, overrides *ConfigOverrides) (*DB, error) {
	db := &postgrespb.DatabaseInstance{}
	if err := res.Unmarshal(resourceRef, db); err != nil {
		return nil, err
	}

	connUri := db.ReplicaConnectionUri
	caCert := db.ReplicaCaCert
	if connUri == "" {
		connUri = db.ConnectionUri
		caCert = db.CaCert
	}

	return newDatabaseFromConnectionURI(ctx, db, connUri, caCert, tp, client, overrides)
}

type ConfigOverrides struct {
	MaxConns                        int32
	MaxConnIdleTime                 time.Duration
	MaxConnLifetimeJitter           time.Duration
	IdleInTransactionSessionTimeout time.Duration
	StatementTimeout                time.Duration
	LockTimeout                     time.Duration
	ConnectTimeout                  time.Duration
	VerifyServerCertificate         bool
}

func NewDatabaseFromConnectionUri(ctx context.Context, db DBInstance, connuri string, tp trace.TracerProvider, client string) (*DB, error) {
	return NewDatabaseFromConnectionUriWithOverrides(ctx, db, connuri, tp, client, nil)
}

func NewDatabaseFromConnectionUriWithOverrides(ctx context.Context, db DBInstance, connuri string, tp trace.TracerProvider, client string, overrides *ConfigOverrides) (*DB, error) {
	var caCert string
	if dbWithCA, ok := db.(interface{ GetCaCert() string }); ok {
		caCert = dbWithCA.GetCaCert()
	}

	return newDatabaseFromConnectionURI(ctx, db, connuri, caCert, tp, client, overrides)
}

func newDatabaseFromConnectionURI(ctx context.Context, db DBInstance, connuri, caCert string, tp trace.TracerProvider, client string, overrides *ConfigOverrides) (*DB, error) {
	config, err := pgxpool.ParseConfig(connuri)
	if err != nil {
		return nil, err
	}

	if overrides != nil && overrides.VerifyServerCertificate {
		if err := configureFullVerification(&config.ConnConfig.Config, caCert); err != nil {
			return nil, err
		}
	}

	var t trace.Tracer
	if tp != nil && db != nil && db.GetEnableTracing() {
		config.ConnConfig.Tracer = otelpgx.NewTracer(otelpgx.WithTracerProvider(tp),
			otelpgx.WithAttributes(semconv.DBNamespace(config.ConnConfig.Database)))
		t = tp.Tracer("namespacelabs.dev/foundation/universe/db/postgres")
	}

	if overrides != nil {
		if overrides.MaxConns > 0 {
			config.MaxConns = overrides.MaxConns
		}

		if overrides.MaxConnIdleTime > 0 {
			config.MaxConnIdleTime = overrides.MaxConnIdleTime
		}

		if overrides.MaxConnLifetimeJitter > 0 {
			config.MaxConnLifetimeJitter = overrides.MaxConnLifetimeJitter
		}

		if overrides.IdleInTransactionSessionTimeout > 0 {
			config.ConnConfig.RuntimeParams["idle_in_transaction_session_timeout"] = fmt.Sprintf("%d", overrides.IdleInTransactionSessionTimeout.Milliseconds())
		}

		if overrides.StatementTimeout > 0 {
			config.ConnConfig.RuntimeParams["statement_timeout"] = fmt.Sprintf("%d", overrides.StatementTimeout.Milliseconds())
		}

		if overrides.LockTimeout > 0 {
			config.ConnConfig.RuntimeParams["lock_timeout"] = fmt.Sprintf("%d", overrides.LockTimeout.Milliseconds())
		}

		if overrides.ConnectTimeout > 0 {
			config.ConnConfig.ConnectTimeout = overrides.ConnectTimeout
		}
	}

	conn, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, err
	}

	return newDatabase(db, conn, t, client), nil
}

func configureFullVerification(config *pgconn.Config, caCert string) error {
	var roots *x509.CertPool
	if caCert != "" {
		roots = x509.NewCertPool()
		if !roots.AppendCertsFromPEM([]byte(caCert)) {
			return errors.New("failed to parse database CA certificate")
		}
	}

	configureTLS := func(config *tls.Config, host string) *tls.Config {
		if config == nil {
			config = &tls.Config{}
		} else {
			config = config.Clone()
		}

		if roots != nil {
			config.RootCAs = roots
		}
		config.InsecureSkipVerify = false
		config.VerifyPeerCertificate = nil
		config.ServerName = host
		return config
	}

	config.TLSConfig = configureTLS(config.TLSConfig, config.Host)
	for _, fallback := range config.Fallbacks {
		fallback.TLSConfig = configureTLS(fallback.TLSConfig, fallback.Host)
	}

	return nil
}
