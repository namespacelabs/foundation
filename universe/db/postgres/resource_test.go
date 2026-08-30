// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package postgres

import (
	"context"
	"testing"
	"time"

	"gotest.tools/assert"
)

func TestMaxConnLifetimeJitterOverride(t *testing.T) {
	db, err := NewDatabaseFromConnectionUriWithOverrides(
		context.Background(),
		nil,
		"postgres://user:password@localhost/database",
		nil,
		"test",
		&ConfigOverrides{MaxConnLifetimeJitter: 10 * time.Minute},
	)
	assert.NilError(t, err)
	t.Cleanup(func() {
		db.PgxPool().Close()
		assert.NilError(t, db.Close())
	})

	assert.Equal(t, db.PgxPool().Config().MaxConnLifetimeJitter, 10*time.Minute)
}
