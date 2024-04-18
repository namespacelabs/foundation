// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package vault_test

import (
	"os"
	"testing"

	"namespacelabs.dev/foundation/universe/vault"
)

func TestParse(t *testing.T) {
	if tb := testBundle(t); tb == nil {
		t.Errorf("expected %T, got nil", tb)
	}
}

func TestParseFromEnv(t *testing.T) {
	os.Setenv(vault.EnvKey, string(testData(t)))
	tb, err := vault.ParseFromEnv()
	if err != nil {
		t.Fatalf("could not parse bundle: %v", err)
	}
	if tb == nil {
		t.Errorf("expected %T, got nil", tb)
	}
}

func TestEncode(t *testing.T) {
	tb := testBundle(t)

	data, err := tb.Encode()
	if err != nil {
		t.Fatalf("could not encode bundle: %v", err)
	}
	if exp, got := 1323, len(data); exp != got {
		t.Errorf("expected %d bytes, got %d", exp, got)
	}
}

func TestCaPool(t *testing.T) {
	tb := testBundle(t)

	pool := tb.CAPool()
	if pool == nil {
		t.Errorf("expected %T, got nil", pool)
	}
}

func TestCertificate(t *testing.T) {
	tb := testBundle(t)

	cert, err := tb.Certificate()
	if err != nil {
		t.Fatalf("could not parse certificate: %v", err)
	}
	if cert.Certificate == nil {
		t.Errorf("expected %T, got nil", cert.Certificate)
	}
	if cert.PrivateKey == nil {
		t.Errorf("expected %T, got nil", cert.Certificate)
	}
}

func TestServerConfig(t *testing.T) {
	tb := testBundle(t)

	config, err := tb.ServerConfig()
	if err != nil {
		t.Fatalf("error getting server config: %v", err)
	}
	if config == nil {
		t.Errorf("expected %T, got nil", config)
	}
}

func ClientrverConfig(t *testing.T) {
	tb := testBundle(t)

	config, err := tb.ClientConfig()
	if err != nil {
		t.Fatalf("error getting client config: %v", err)
	}
	if config == nil {
		t.Errorf("expected %T, got nil", config)
	}
}

func testBundle(t *testing.T) *vault.TlsBundle {
	tb, err := vault.Parse(testData(t))
	if err != nil {
		t.Fatalf("could not parse bundle: %v", err)
	}
	return tb
}

func testData(t *testing.T) []byte {
	const path = "../../internal/testdata/vault/tls_bundle.json"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("could not parse %q: %v", path, err)
	}
	return data
}
