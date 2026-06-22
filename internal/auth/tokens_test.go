// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package auth

import (
	"os"
	"path/filepath"
	"testing"

	"namespacelabs.dev/foundation/internal/workspace/dirs"
)

// useTempConfigDir redirects dirs.Config() at a temporary directory for the
// duration of the test, working across both the darwin and XDG resolutions of
// os.UserConfigDir().
func useTempConfigDir(t *testing.T) string {
	t.Helper()

	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("XDG_CONFIG_HOME", tmp)

	// Reset selectors that influence the token file location.
	Workspace = ""
	Keychain = ""

	dir, err := dirs.Config()
	if err != nil {
		t.Fatalf("dirs.Config(): %v", err)
	}

	return dir
}

func TestStoreTokenInvalidatesCache(t *testing.T) {
	configDir := useTempConfigDir(t)

	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	cachePath := filepath.Join(configDir, tokenCacheLoc)
	if err := os.WriteFile(cachePath, []byte("stale-derived-tenant-token"), 0o600); err != nil {
		t.Fatalf("seed cache: %v", err)
	}

	// Simulate `nsc login` storing a fresh session token.
	if err := StoreToken(StoredToken{SessionToken: "nss_new-session"}); err != nil {
		t.Fatalf("StoreToken: %v", err)
	}

	if _, err := os.Stat(cachePath); !os.IsNotExist(err) {
		t.Fatalf("expected token.cache to be removed after StoreToken, stat err = %v", err)
	}

	if _, err := os.Stat(filepath.Join(configDir, defaultTokenLoc)); err != nil {
		t.Fatalf("expected token.json to be written: %v", err)
	}
}

func TestDeleteStoredTokenInvalidatesCache(t *testing.T) {
	configDir := useTempConfigDir(t)

	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	tokenPath := filepath.Join(configDir, defaultTokenLoc)
	cachePath := filepath.Join(configDir, tokenCacheLoc)
	if err := os.WriteFile(tokenPath, []byte(`{"session_token":"nss_x"}`), 0o600); err != nil {
		t.Fatalf("seed token: %v", err)
	}
	if err := os.WriteFile(cachePath, []byte("stale-derived-tenant-token"), 0o600); err != nil {
		t.Fatalf("seed cache: %v", err)
	}

	// Simulate `nsc logout`.
	if err := DeleteStoredToken(); err != nil {
		t.Fatalf("DeleteStoredToken: %v", err)
	}

	if _, err := os.Stat(cachePath); !os.IsNotExist(err) {
		t.Fatalf("expected token.cache to be removed after logout, stat err = %v", err)
	}
	if _, err := os.Stat(tokenPath); !os.IsNotExist(err) {
		t.Fatalf("expected token.json to be removed after logout, stat err = %v", err)
	}
}

// DeleteStoredToken must drop a stale cache even when the token file is already
// gone, so a derived token can't outlive logout.
func TestDeleteStoredTokenInvalidatesCacheWithoutTokenFile(t *testing.T) {
	configDir := useTempConfigDir(t)

	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	cachePath := filepath.Join(configDir, tokenCacheLoc)
	if err := os.WriteFile(cachePath, []byte("stale-derived-tenant-token"), 0o600); err != nil {
		t.Fatalf("seed cache: %v", err)
	}

	if err := DeleteStoredToken(); err != nil {
		t.Fatalf("DeleteStoredToken: %v", err)
	}

	if _, err := os.Stat(cachePath); !os.IsNotExist(err) {
		t.Fatalf("expected token.cache to be removed, stat err = %v", err)
	}
}
