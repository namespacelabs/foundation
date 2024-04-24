// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package vault_test

import (
	"os"
	"testing"

	"namespacelabs.dev/foundation/universe/vault"
)

func TestParseCredentials(t *testing.T) {
	if c := testCredentials(t); c == nil {
		t.Errorf("expected %T, got nil", c)
	}
}

func TestParseFromEnv(t *testing.T) {
	key := "VAULT_CREDENTIALS"
	os.Setenv(key, string(testCredentialsData(t)))
	c, err := vault.ParseCredentialsFromEnv(key)
	if err != nil {
		t.Fatalf("could not parse bundle: %v", err)
	}
	if c == nil {
		t.Errorf("expected %T, got nil", c)
	}
}

func TestEncode(t *testing.T) {
	c := testCredentials(t)

	data, err := c.Encode()
	if err != nil {
		t.Fatalf("could not encode bundle: %v", err)
	}
	if exp, got := 44, len(data); exp != got {
		t.Errorf("expected %d bytes, got %d", exp, got)
	}
}

func testCredentials(t *testing.T) *vault.Credentials {
	c, err := vault.ParseCredentials(testCredentialsData(t))
	if err != nil {
		t.Fatalf("could not parse bundle: %v", err)
	}
	return c
}

func testCredentialsData(t *testing.T) []byte {
	const path = "testdata/credentials.json"
	data, err := lib.ReadFile(path)
	if err != nil {
		t.Fatalf("could not parse %q: %v", path, err)
	}
	return data
}
