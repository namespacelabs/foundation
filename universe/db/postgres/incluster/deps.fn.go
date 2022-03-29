// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

// This file was automatically generated.
package incluster

import (
	"context"

	"github.com/jackc/pgx/v4/pgxpool"
	"namespacelabs.dev/foundation/std/go/core"
	"namespacelabs.dev/foundation/universe/db/postgres/creds"
)

type ExtensionDeps struct {
	Creds          *creds.Creds
	ReadinessCheck core.Check
}

type _checkProvideDatabase func(context.Context, string, *Database, ExtensionDeps) (*pgxpool.Pool, error)

var _ _checkProvideDatabase = ProvideDatabase