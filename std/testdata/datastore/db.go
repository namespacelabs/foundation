// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package datastore

import (
	"context"

	fninit "namespacelabs.dev/foundation/std/go/core/init"
)

type DB struct{}

func ProvideDatabase(_ context.Context, _ fninit.Caller, _ *Database, deps *SingletonDeps) (*DB, error) {
	deps.ReadinessCheck.RegisterFunc("foobar", func(ctx context.Context) error {
		return nil
	})

	return &DB{}, nil
}
