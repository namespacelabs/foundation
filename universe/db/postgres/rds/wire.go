// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package rds

import (
	"context"

	"namespacelabs.dev/foundation/universe/db/postgres"
	"namespacelabs.dev/foundation/universe/db/postgres/incluster"
)

func ProvideDatabase(ctx context.Context, db *Database, deps ExtensionDeps) (*postgres.DB, error) {
	endpoint, err := incluster.GetEndpoint()
	if err != nil {
		return nil, err
	}

	if endpoint != nil {
		return incluster.ProvideDb(ctx, db.Name, db.SchemaFile, endpoint, deps.Creds, deps.Wire)
	}

	// TODO ??
	return nil, nil
}
