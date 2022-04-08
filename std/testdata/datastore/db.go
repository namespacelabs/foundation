// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package datastore

import "context"

type DB struct{}

func ProvideDatabase(_ context.Context, _ string, _ *Database, deps *SingletonDeps, _ *DatabaseDeps) (*DB, error) {
	deps.ReadinessCheck.RegisterFunc("foobar", func(ctx context.Context) error {
		return nil
	})

	return &DB{}, nil
}
