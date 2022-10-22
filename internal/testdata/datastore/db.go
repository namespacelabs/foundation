// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package datastore

import (
	"context"
)

type DB struct{}

func ProvideDatabase(_ context.Context, _ *Database, deps ExtensionDeps) (*DB, error) {
	deps.ReadinessCheck.RegisterFunc("foobar", func(ctx context.Context) error {
		return nil
	})

	return &DB{}, nil
}
