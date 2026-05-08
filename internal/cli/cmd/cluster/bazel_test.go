// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"strings"
	"testing"
	"time"

	bazelv1beta "buf.build/gen/go/namespace/cloud/protocolbuffers/go/proto/namespace/cloud/integrations/bazel/v1beta"
)

func TestBaseBazelSetup(t *testing.T) {
	t.Parallel()

	expiresAt := time.Now()
	response := &bazelv1beta.EnsureBazelCacheResponse{
		CacheEndpoint:           "grpcs://cache.example:444",
		HttpsCacheEndpoint:      "grpcs://ingress.example:443",
		CredentialHelperDomains: []string{"api.example.com"},
	}

	t.Run("uses credential helper config by default", func(t *testing.T) {
		t.Parallel()

		out := baseBazelSetup(response, &expiresAt)
		if out.Endpoint != response.GetHttpsCacheEndpoint() {
			t.Fatalf("unexpected endpoint: %q", out.Endpoint)
		}
		if len(out.CredentialHelperDomains) != 1 || out.CredentialHelperDomains[0] != "api.example.com" {
			t.Fatalf("unexpected credential helper domains: %v", out.CredentialHelperDomains)
		}
	})

	t.Run("uses direct cache endpoint when response requires workload mtls", func(t *testing.T) {
		t.Parallel()

		response := &bazelv1beta.EnsureBazelCacheResponse{
			CacheEndpoint:           response.GetCacheEndpoint(),
			HttpsCacheEndpoint:      response.GetHttpsCacheEndpoint(),
			CredentialHelperDomains: append([]string(nil), response.GetCredentialHelperDomains()...),
		}
		response.SetUseWorkloadMtls(true)

		out := baseBazelSetup(response, &expiresAt)
		if out.Endpoint != response.GetCacheEndpoint() {
			t.Fatalf("unexpected endpoint: %q", out.Endpoint)
		}
		if len(out.CredentialHelperDomains) != 0 {
			t.Fatalf("unexpected credential helper domains: %v", out.CredentialHelperDomains)
		}
	})
}

func TestMakeEnsureBazelCacheRequest(t *testing.T) {
	t.Parallel()

	msg := makeEnsureBazelCacheRequest(7, true, false, "amp-test")

	if got := msg.GetVersion(); got != 7 {
		t.Fatalf("unexpected version: %d", got)
	}
	if !msg.GetExperimentalDirectMtls() {
		t.Fatal("expected experimental_direct_mtls to be set")
	}
	if msg.GetEnableRemoteAssetApi() {
		t.Fatal("expected enable_remote_asset to be false")
	}
	if got := msg.GetExperimentalCacheName(); got != "amp-test" {
		t.Fatalf("unexpected experimental cache name: %q", got)
	}
}

func TestMakeEnsureBazelCacheRequestWithRemoteAsset(t *testing.T) {
	t.Parallel()

	msg := makeEnsureBazelCacheRequest(7, true, true, "amp-test")

	if got := msg.GetVersion(); got != 7 {
		t.Fatalf("unexpected version: %d", got)
	}
	if !msg.GetExperimentalDirectMtls() {
		t.Fatal("expected experimental_direct_mtls to be set")
	}
	if !msg.GetEnableRemoteAssetApi() {
		t.Fatal("expected enable_remote_asset to be set")
	}
	if got := msg.GetExperimentalCacheName(); got != "amp-test" {
		t.Fatalf("unexpected experimental cache name: %q", got)
	}
}

func TestToBazelConfigExperimentalDirect(t *testing.T) {
	t.Parallel()

	config, err := toBazelConfig(context.Background(), bazelSetup{
		Endpoint:     "grpcs://cache.example:444",
		ServerCaCert: "/tmp/server_ca.cert",
		ClientCert:   "/tmp/client.cert",
		ClientKey:    "/tmp/client.key",
	}, false, "build")
	if err != nil {
		t.Fatalf("toBazelConfig: %v", err)
	}

	got := string(config)
	for _, want := range []string{
		"build --remote_cache=grpcs://cache.example:444\n",
		"build --tls_certificate=/tmp/server_ca.cert\n",
		"build --tls_client_certificate=/tmp/client.cert\n",
		"build --tls_client_key=/tmp/client.key\n",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing config line %q in %q", want, got)
		}
	}

	if strings.Contains(got, "credential_helper") {
		t.Fatalf("unexpected credential helper config: %q", got)
	}
}

func TestConvertECPrivateKeyToPKCS8(t *testing.T) {
	t.Parallel()

	privateKeyPem, _, err := genPrivAndPublicKeysPEM()
	if err != nil {
		t.Fatalf("genPrivAndPublicKeysPEM: %v", err)
	}

	converted, err := convertECPrivateKeyToPKCS8(privateKeyPem)
	if err != nil {
		t.Fatalf("convertECPrivateKeyToPKCS8: %v", err)
	}

	got := string(converted)
	if !strings.Contains(got, "BEGIN PRIVATE KEY") {
		t.Fatalf("expected PKCS#8 private key, got %q", got)
	}
	if strings.Contains(got, "BEGIN EC PRIVATE KEY") {
		t.Fatalf("unexpected EC private key format: %q", got)
	}
}
