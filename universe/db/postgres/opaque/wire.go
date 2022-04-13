// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package opaque

import (
	"context"

	"github.com/jackc/pgx/v4/pgxpool"
	"namespacelabs.dev/foundation/universe/db/postgres"
)

func ProvideDatabase(ctx context.Context, db *Database, single ExtensionDeps, deps DatabaseDeps) (*pgxpool.Pool, error) {
	return postgres.ProvideDatabase(ctx, db, deps.Creds.Username, deps.Creds.Password, single.ReadinessCheck)
}
