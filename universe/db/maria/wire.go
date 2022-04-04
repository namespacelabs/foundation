// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package postgres

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
	"namespacelabs.dev/foundation/std/go/core"
	"namespacelabs.dev/foundation/universe/db/postgres/creds"
)

func logf(message string, args ...interface{}) {
	fmt.Fprintf(os.Stdout, "%s : %s\n", time.Now().String(), fmt.Sprintf(message, args...))
}

func ProvideDatabase(ctx context.Context, caller string, db *Database, creds *creds.Creds, ready core.Check) (*pgxpool.Pool, error) {
	// Config has to be created by ParseConfig
	config, err := pgxpool.ParseConfig(fmt.Sprintf("postgres://%s:%s@%s:%d/%s",
		creds.Username,
		creds.Password,
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
	ready.RegisterFunc(fmt.Sprintf("%s/%s", caller, db.Name), func(ctx context.Context) error {
		return conn.Ping(ctx)
	})

	return conn, nil
}
