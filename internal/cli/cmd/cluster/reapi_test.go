// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestReapiSetupCommandFlags(t *testing.T) {
	t.Parallel()

	for _, flavor := range []string{reapiFlavorBazel, reapiFlavorBuck2} {
		setup, args, err := NewReapiCmd().Find([]string{"setup", flavor})
		if err != nil || len(args) != 0 {
			t.Fatalf("Find(setup %s) = (%v, %v)", flavor, args, err)
		}
		for _, name := range []string{"config", "output", "key", "token", "static", "remote", "storage", "static_token_duration"} {
			if setup.Flag(name) == nil {
				t.Errorf("setup %s is missing --%s", flavor, name)
			}
		}
		if setup.Flag("flavor") != nil || setup.Flag("execution") != nil {
			t.Errorf("setup %s exposes a legacy flavor or execution flag", flavor)
		}
		if got := setup.Flag("remote").DefValue; got != "true" {
			t.Errorf("setup %s --remote default = %q, want true", flavor, got)
		}
	}

	bazel, _, _ := NewReapiCmd().Find([]string{"setup", reapiFlavorBazel})
	for _, name := range []string{"command", "enable_remote_asset_api", "disable_build_events"} {
		if bazel.Flag(name) == nil {
			t.Errorf("setup bazel is missing --%s", name)
		}
	}
	buck2, _, _ := NewReapiCmd().Find([]string{"setup", reapiFlavorBuck2})
	for _, name := range []string{"command", "enable_remote_asset_api", "disable_build_events"} {
		if buck2.Flag(name) != nil {
			t.Errorf("setup buck2 unexpectedly exposes --%s", name)
		}
	}

	createToken, args, err := NewReapiCmd().Find([]string{"create-token"})
	if err != nil || len(args) != 0 {
		t.Fatalf("Find(create-token) = (%v, %v)", args, err)
	}
	for _, name := range []string{"token", "expires_in", "scope"} {
		if createToken.Flag(name) == nil {
			t.Errorf("create-token is missing --%s", name)
		}
	}
}

func TestBuck2CacheSetupCommand(t *testing.T) {
	t.Parallel()
	setup, args, err := NewBuck2CacheCmd().Find([]string{"setup"})
	if err != nil || len(args) != 0 {
		t.Fatalf("Find(setup) = (%v, %v)", args, err)
	}
	if setup.Flag("remote") != nil {
		t.Error("cache setup must not expose --remote")
	}
}

func TestValidateReapiSetupFlags(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name                 string
		output               string
		storage              string
		duration             time.Duration
		remote               bool
		enableRemoteAssetAPI bool
		wantErr              string
	}{
		{name: "remote execution", output: "plain", storage: "read-write", duration: time.Hour, remote: true},
		{name: "storage only", output: "json", storage: "read-only", duration: time.Hour},
		{name: "invalid output", output: "yaml", storage: "read-write", duration: time.Hour, wantErr: "unsupported output"},
		{name: "invalid storage", output: "plain", storage: "invalid", duration: time.Hour, wantErr: "invalid storage mode"},
		{name: "read-only execution", output: "plain", storage: "read-only", duration: time.Hour, remote: true, wantErr: "requires --remote=false"},
		{name: "invalid duration", output: "plain", storage: "read-write", wantErr: "must be greater than zero"},
		{name: "read-only remote asset", output: "plain", storage: "read-only", duration: time.Hour, enableRemoteAssetAPI: true, wantErr: "may not be used"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := validateReapiSetupFlags(tc.output, tc.storage, tc.duration, tc.remote, tc.enableRemoteAssetAPI)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("validateReapiSetupFlags: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %v, want error containing %q", err, tc.wantErr)
			}
		})
	}
}

func TestBazelCommandVisibility(t *testing.T) {
	t.Parallel()
	bazel := NewBazelCmd()
	setup, _, err := bazel.Find([]string{"setup"})
	if err != nil || setup.Hidden {
		t.Fatalf("bazel setup should be visible: command=%v err=%v", setup, err)
	}
	cache, _, err := bazel.Find([]string{"cache"})
	if err != nil || !cache.Hidden {
		t.Fatalf("bazel cache should be hidden: command=%v err=%v", cache, err)
	}
}

func TestReapiReadinessConfig(t *testing.T) {
	t.Parallel()

	static, cleanup, err := reapiReadinessConfig(reapiSetupResult{bazelRbeSetup: bazelRbeSetup{
		StorageEndpoint:  "grpcs://storage.example:443",
		IngressAuthToken: "token",
	}})
	if err != nil {
		t.Fatalf("reapiReadinessConfig(static): %v", err)
	}
	defer cleanup()
	if static.endpoint != "grpcs://storage.example:443" || static.bearerToken != "token" || static.clientCert != "" || static.clientKey != "" {
		t.Fatalf("static readiness = %#v", static)
	}

	mtls, cleanup, err := reapiReadinessConfig(reapiSetupResult{
		bazelRbeSetup: bazelRbeSetup{StorageEndpoint: "grpcs://storage.example:443"},
		clientCertPEM: []byte("certificate"),
		clientKeyPEM:  []byte("private-key"),
	})
	if err != nil {
		t.Fatalf("reapiReadinessConfig(mTLS): %v", err)
	}
	defer cleanup()
	assertCredentialFile(t, mtls.clientCert, "certificate")
	assertCredentialFile(t, mtls.clientKey, "private-key")

	if _, _, err := reapiReadinessConfig(reapiSetupResult{clientCertPEM: []byte("certificate")}); err == nil {
		t.Fatal("certificate without key succeeded")
	}
	if _, _, err := reapiReadinessConfig(reapiSetupResult{clientKeyPEM: []byte("private-key")}); err == nil {
		t.Fatal("key without certificate succeeded")
	}
}

func TestEmitReapiBazelConfig(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "namespace.bazelrc")
	result := reapiSetupResult{bazelRbeSetup: bazelRbeSetup{
		SchedulerEndpoint: "grpcs://scheduler.example:443",
		StorageEndpoint:   "grpcs://storage.example:443",
		IngressAuthToken:  "token",
	}}
	if err := emitReapiBazelConfig(context.Background(), result, configPath, "json", "build", true, true); err != nil {
		t.Fatalf("emitReapiBazelConfig: %v", err)
	}

	config, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	for _, want := range []string{
		"build --remote_executor=grpcs://scheduler.example:443\n",
		"build --remote_cache=grpcs://storage.example:443\n",
		"build --remote_header=x-nsc-ingress-auth=Bearer\\ token\n",
	} {
		if !strings.Contains(string(config), want) {
			t.Errorf("config does not contain %q:\n%s", want, config)
		}
	}
	assertFileMode(t, configPath, 0600)
}

func TestEmitReapiBuck2Config(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), ".buckconfig.local")
	result := reapiSetupResult{
		bazelRbeSetup: bazelRbeSetup{
			SchedulerEndpoint: "grpcs://scheduler.example:443",
			StorageEndpoint:   "grpcs://storage.example:443",
		},
		clientCertPEM: []byte("certificate"),
		clientKeyPEM:  []byte("private-key"),
	}
	if err := emitReapiBuck2Config(context.Background(), result, configPath, "json", true, time.Hour); err != nil {
		t.Fatalf("emitReapiBuck2Config: %v", err)
	}

	identityPath, err := filepath.Abs(configPath + ".tls.pem")
	if err != nil {
		t.Fatalf("filepath.Abs: %v", err)
	}
	assertCredentialFile(t, identityPath, "certificate\nprivate-key")
	config, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	for _, want := range []string{
		"engine_address = grpc://scheduler.example:443\n",
		"action_cache_address = grpc://storage.example:443\n",
		"tls_client_cert = " + identityPath + "\n",
	} {
		if !strings.Contains(string(config), want) {
			t.Errorf("config does not contain %q:\n%s", want, config)
		}
	}
}

func TestEmitReapiBuck2ConfigDefaultPathKeepsIdentityOutsideConfigDirectory(t *testing.T) {
	home := t.TempDir()
	configHome := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", configHome)
	configPath := filepath.Join(home, ".buckconfig.d", "50-namespace")

	result := reapiSetupResult{
		bazelRbeSetup: bazelRbeSetup{
			SchedulerEndpoint: "grpcs://scheduler.example:443",
			StorageEndpoint:   "grpcs://storage.example:443",
		},
		clientCertPEM: []byte("certificate"),
		clientKeyPEM:  []byte("private-key"),
	}
	if err := emitReapiBuck2Config(context.Background(), result, "", "json", true, time.Hour); err != nil {
		t.Fatalf("emitReapiBuck2Config: %v", err)
	}

	identityPath := filepath.Join(configHome, "ns", "buck2", "client.pem")
	assertCredentialFile(t, identityPath, "certificate\nprivate-key")
	config, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if want := "tls_client_cert = " + identityPath + "\n"; !strings.Contains(string(config), want) {
		t.Fatalf("config does not contain %q:\n%s", want, config)
	}

	result.clientCertPEM = []byte("replacement-certificate")
	result.clientKeyPEM = []byte("replacement-private-key")
	if err := emitReapiBuck2Config(context.Background(), result, "", "json", true, time.Hour); err != nil {
		t.Fatalf("second emitReapiBuck2Config: %v", err)
	}
	assertCredentialFile(t, identityPath, "replacement-certificate\nreplacement-private-key")
}

func assertFileMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat(%q): %v", path, err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("mode of %q = %o, want %o", path, got, want)
	}
}
