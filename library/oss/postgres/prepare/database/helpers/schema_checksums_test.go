// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package helpers

import (
	"testing"

	"namespacelabs.dev/foundation/schema"
)

func TestSchemaChecksum(t *testing.T) {
	a := schemaChecksum([]byte("CREATE TABLE foo();"))
	if a != schemaChecksum([]byte("CREATE TABLE foo();")) {
		t.Fatal("checksum is not deterministic")
	}

	if a == schemaChecksum([]byte("CREATE TABLE foo() ;")) {
		t.Fatal("checksum should change when bytes change")
	}

	const wantPrefix = "sha256:"
	if got := schemaChecksum(nil); got[:len(wantPrefix)] != wantPrefix {
		t.Fatalf("checksum %q missing %q prefix", got, wantPrefix)
	}
}

func TestAdvisoryLockKey(t *testing.T) {
	if advisoryLockKey("db") != advisoryLockKey("db") {
		t.Fatal("advisory lock key is not deterministic")
	}

	if advisoryLockKey("db_a") == advisoryLockKey("db_b") {
		t.Fatal("advisory lock key should differ per database")
	}
}

func TestValidateSchemaPaths(t *testing.T) {
	for _, tc := range []struct {
		name    string
		paths   []string
		wantErr bool
	}{
		{name: "ok", paths: []string{"a.sql", "b.sql"}},
		{name: "empty", paths: []string{""}, wantErr: true},
		{name: "duplicate", paths: []string{"a.sql", "a.sql"}, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			schemas := make([]*schema.FileContents, len(tc.paths))
			for i, p := range tc.paths {
				schemas[i] = &schema.FileContents{Path: p}
			}

			if err := validateSchemaPaths(schemas); (err != nil) != tc.wantErr {
				t.Fatalf("validateSchemaPaths(%v) error = %v, wantErr %v", tc.paths, err, tc.wantErr)
			}
		})
	}
}
