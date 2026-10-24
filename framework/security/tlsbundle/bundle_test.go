// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package tlsbundle_test

import (
	"embed"
	"os"
	"testing"

	"namespacelabs.dev/foundation/framework/security/tlsbundle"
)

//go:embed testdata/*.json
var lib embed.FS

func TestParseTlsBundle(t *testing.T) {
	if tb := testBundle(t); tb == nil {
		t.Errorf("expected %T, got nil", tb)
	}
}

func TestParseTlsBundleFromEnv(t *testing.T) {
	key := "TLS_BUNDLE"
	os.Setenv(key, string(testBundleData(t)))
	tb, err := tlsbundle.ParseTlsBundleFromEnv(key)
	if err != nil {
		t.Fatalf("could not parse bundle: %v", err)
	}
	if tb == nil {
		t.Errorf("expected %T, got nil", tb)
	}
}

func TestTlsBundleEncode(t *testing.T) {
	tb := testBundle(t)

	data, err := tb.Encode()
	if err != nil {
		t.Fatalf("could not encode bundle: %v", err)
	}
	if exp, got := 1323, len(data); exp != got {
		t.Errorf("expected %d bytes, got %d", exp, got)
	}
}

func testBundle(t *testing.T) *tlsbundle.TlsBundle {
	tb, err := tlsbundle.ParseTlsBundle(testBundleData(t))
	if err != nil {
		t.Fatalf("could not parse bundle: %v", err)
	}
	return tb
}

func testBundleData(t *testing.T) []byte {
	const path = "testdata/tls_bundle.json"
	data, err := lib.ReadFile(path)
	if err != nil {
		t.Fatalf("could not parse %q: %v", path, err)
	}
	return data
}
