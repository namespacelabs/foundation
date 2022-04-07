// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package opaque

import (
	"context"

	"github.com/jackc/pgx/v4/pgxpool"
	"namespacelabs.dev/foundation/universe/db/postgres"
)

func ProvideDatabase(ctx context.Context, caller string, db *Database, sdeps SingletonDeps, dbdeps DatabaseDeps) (*pgxpool.Pool, error) {
	return postgres.ProvideDatabase(ctx, caller, db, dbdeps.Creds.Username, dbdeps.Creds.Password, sdeps.ReadinessCheck)
}
