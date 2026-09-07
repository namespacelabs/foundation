// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestResolveBuck2ConfigPath(t *testing.T) {
	t.Parallel()

	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("no home directory: %v", err)
	}

	path, isDefault, err := resolveBuck2ConfigPath("")
	if err != nil {
		t.Fatalf("resolveBuck2ConfigPath: %v", err)
	}

	// buck2 reads ~/.buckconfig.d, so the default must land there rather than
	// in a temporary directory.
	if want := filepath.Join(home, ".buckconfig.d", "50-namespace"); path != want {
		t.Errorf("default path = %q, want %q", path, want)
	}
	if !isDefault {
		t.Error("expected the resolved path to be reported as the default")
	}

	path, isDefault, err = resolveBuck2ConfigPath("/tmp/explicit")
	if err != nil {
		t.Fatalf("resolveBuck2ConfigPath: %v", err)
	}
	if path != "/tmp/explicit" || isDefault {
		t.Errorf("explicit path = (%q, %t), want (\"/tmp/explicit\", false)", path, isDefault)
	}
}

func TestBuck2Address(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		endpoint string
		address  string
		tls      bool
		wantErr  bool
	}{
		{endpoint: "grpcs://storage.example:443", address: "grpc://storage.example:443", tls: true},
		{endpoint: "https://storage.example:443", address: "grpc://storage.example:443", tls: true},
		{endpoint: "grpc://localhost:8980", address: "grpc://localhost:8980", tls: false},
		{endpoint: "http://localhost:8980", address: "grpc://localhost:8980", tls: false},
		{endpoint: "storage.example:443", address: "grpc://storage.example:443", tls: true},
		{endpoint: "unix://storage.example", wantErr: true},
		{endpoint: "grpcs://", wantErr: true},
	} {
		address, tls, err := buck2Address(tc.endpoint)
		if tc.wantErr {
			if err == nil {
				t.Errorf("buck2Address(%q) = (%q, %t), want error", tc.endpoint, address, tls)
			}
			continue
		}

		if err != nil {
			t.Errorf("buck2Address(%q): %v", tc.endpoint, err)
			continue
		}

		if address != tc.address || tls != tc.tls {
			t.Errorf("buck2Address(%q) = (%q, %t), want (%q, %t)", tc.endpoint, address, tls, tc.address, tc.tls)
		}
	}
}

func TestToBuck2ConfigRemoteExecution(t *testing.T) {
	t.Parallel()

	out, err := buck2ExecutionSetup("grpcs://scheduler.example:443", "grpcs://storage.example:443", "")
	if err != nil {
		t.Fatalf("buck2ExecutionSetup: %v", err)
	}

	out.StaticToken = "tok123"
	out.GrpcTimeout = 5 * time.Minute

	config, err := toBuck2Config(out)
	if err != nil {
		t.Fatalf("toBuck2Config: %v", err)
	}

	got := string(config)
	for _, want := range []string{
		"[buck2_re_client]\n",
		"engine_address = grpc://scheduler.example:443\n",
		"action_cache_address = grpc://storage.example:443\n",
		"cas_address = grpc://storage.example:443\n",
		"tls = true\n",
		"grpc_timeout = 300\n",
		"http_headers = x-nsc-ingress-auth:Bearer tok123\n",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing config line %q in %q", want, got)
		}
	}

	// buck2 rejects grpcs:// outright, so it must not leak into the config.
	if strings.Contains(got, "grpcs://") {
		t.Fatalf("config retained a grpcs:// endpoint: %q", got)
	}
}

func TestToBuck2ConfigCacheOnly(t *testing.T) {
	t.Parallel()

	out, err := buck2CacheOnlySetup("grpcs://ingress.example:443", "main")
	if err != nil {
		t.Fatalf("buck2CacheOnlySetup: %v", err)
	}

	out.StaticToken = "tok123"

	config, err := toBuck2Config(out)
	if err != nil {
		t.Fatalf("toBuck2Config: %v", err)
	}

	got := string(config)
	for _, want := range []string{
		// buck2 does not start without an engine address, even when only the
		// cache is used.
		"engine_address = grpc://ingress.example:443\n",
		"action_cache_address = grpc://ingress.example:443\n",
		"cas_address = grpc://ingress.example:443\n",
		"instance_name = main\n",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing config line %q in %q", want, got)
		}
	}

	if strings.Contains(got, "grpc_timeout") {
		t.Fatalf("cache-only config must not set a timeout it did not receive: %q", got)
	}
}

func TestBuck2ExecutionSetupRejectsMixedTLS(t *testing.T) {
	t.Parallel()

	if _, err := buck2ExecutionSetup("grpcs://scheduler.example:443", "grpc://storage.example:8980", ""); err == nil {
		t.Fatal("expected mixed TLS endpoints to be rejected")
	}
}

func TestToBuck2ConfigRejectsUnusableTokens(t *testing.T) {
	t.Parallel()

	base, err := buck2CacheOnlySetup("grpcs://ingress.example:443", "")
	if err != nil {
		t.Fatalf("buck2CacheOnlySetup: %v", err)
	}

	for _, token := range []string{"tok,123", "tok$HOME", "tok\n123"} {
		out := base
		out.StaticToken = token

		if _, err := toBuck2Config(out); err == nil {
			t.Errorf("expected token %q to be rejected", token)
		}
	}

	out := base
	out.StaticToken = "tok-123_abc.def$"
	if _, err := toBuck2Config(out); err != nil {
		t.Errorf("token with a trailing dollar sign is safe, but was rejected: %v", err)
	}
}
