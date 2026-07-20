// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	iamv1beta "buf.build/gen/go/namespace/cloud/protocolbuffers/go/proto/namespace/cloud/iam/v1beta"
	bazelv1beta "buf.build/gen/go/namespace/cloud/protocolbuffers/go/proto/namespace/cloud/integrations/bazel/v1beta"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestNewBazelCreateTokenCmd(t *testing.T) {
	t.Parallel()

	cmd := NewBazelCmd()
	createToken, _, err := cmd.Find([]string{"create-token"})
	if err != nil {
		t.Fatalf("Find(create-token): %v", err)
	}

	for name, wantDefault := range map[string]string{
		"token":      "token.json",
		"expires_in": "2160h0m0s",
		"scope":      "user",
	} {
		flag := createToken.Flags().Lookup(name)
		if flag == nil {
			t.Fatalf("bazel create-token is missing --%s", name)
		}
		if flag.DefValue != wantDefault {
			t.Fatalf("--%s default = %q, want %q", name, flag.DefValue, wantDefault)
		}
	}
	if createToken.Flags().Lookup("name") != nil {
		t.Fatal("bazel create-token must not expose --name")
	}
}

func TestBazelCacheSetupAcceptsToken(t *testing.T) {
	t.Parallel()

	cmd := NewBazelCmd()
	setup, _, err := cmd.Find([]string{"cache", "setup"})
	if err != nil {
		t.Fatalf("Find(cache setup): %v", err)
	}
	if setup.Flags().Lookup("token") == nil {
		t.Fatal("bazel cache setup is missing --token")
	}
	if setup.Flags().Lookup("disable_build_events") == nil {
		t.Fatal("bazel cache setup is missing --disable_build_events")
	}
}

func TestNewBazelInvocationListCmd(t *testing.T) {
	t.Parallel()

	cmd := NewBazelCmd()
	list, _, err := cmd.Find([]string{"invocation", "list"})
	if err != nil {
		t.Fatalf("Find(invocation list): %v", err)
	}

	for name, wantDefault := range map[string]string{
		"max_entries": "50",
		"output":      "plain",
		"since":       "0s",
	} {
		flag := list.Flags().Lookup(name)
		if flag == nil {
			t.Fatalf("bazel invocation list is missing --%s", name)
		}
		if flag.DefValue != wantDefault {
			t.Fatalf("--%s default = %q, want %q", name, flag.DefValue, wantDefault)
		}
	}
}

func TestWriteBazelInvocationList(t *testing.T) {
	t.Parallel()

	response := &bazelv1beta.ListInvocationsResponse{
		Invocations: []*bazelv1beta.InvocationListEntry{
			{
				InvocationId: "newest",
				ProjectId:    "project-b",
				BuildId:      "build-b",
				StartedAt:    timestamppb.New(time.Date(2026, time.July, 17, 12, 0, 0, 0, time.UTC)),
			},
			{
				InvocationId: "older",
				ProjectId:    "project-a",
				BuildId:      "build-a",
				StartedAt:    timestamppb.New(time.Date(2026, time.July, 17, 11, 0, 0, 0, time.UTC)),
				CompletedAt:  timestamppb.New(time.Date(2026, time.July, 17, 11, 5, 0, 0, time.UTC)),
			},
		},
	}

	t.Run("plain", func(t *testing.T) {
		rows := bazelInvocationRows(response.GetInvocations())
		if len(rows) != 2 {
			t.Fatalf("rows = %#v, want two entries", rows)
		}
		if rows[0]["invocation_id"] != "newest" || rows[1]["invocation_id"] != "older" {
			t.Fatalf("rows changed invocation order: %#v", rows)
		}
		if rows[0]["completed_at"] != "-" || rows[1]["completed_at"] == "-" {
			t.Fatalf("completed times = %q, %q, want missing then populated", rows[0]["completed_at"], rows[1]["completed_at"])
		}
		if _, ok := rows[0]["project_id"]; ok {
			t.Fatalf("plain output contains project_id: %#v", rows[0])
		}
		if _, ok := rows[0]["build_id"]; ok {
			t.Fatalf("plain output contains build_id: %#v", rows[0])
		}
	})

	t.Run("json", func(t *testing.T) {
		var output bytes.Buffer
		if err := writeBazelInvocationListJSON(&output, response.GetInvocations()); err != nil {
			t.Fatalf("writeBazelInvocationListJSON: %v", err)
		}

		var got []map[string]any
		if err := json.Unmarshal(output.Bytes(), &got); err != nil {
			t.Fatalf("unmarshal JSON output: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("JSON output = %#v, want two entries", got)
		}
		if got[0]["invocation_id"] != "newest" || got[0]["started_at"] != "2026-07-17T12:00:00Z" {
			t.Fatalf("first invocation = %#v, want newest invocation", got[0])
		}
		if _, ok := got[0]["completed_at"]; ok {
			t.Fatalf("running invocation contains completed_at: %#v", got[0])
		}
		if _, ok := got[0]["project_id"]; ok {
			t.Fatalf("JSON output contains project_id: %#v", got[0])
		}
		if _, ok := got[0]["build_id"]; ok {
			t.Fatalf("JSON output contains build_id: %#v", got[0])
		}
	})
}

func TestNewBazelTokenRequest(t *testing.T) {
	t.Parallel()

	expiresAt := time.Now().Add(30 * 24 * time.Hour)
	req, err := newBazelTokenRequest("rbe-ci-token", expiresAt, "user")
	if err != nil {
		t.Fatalf("newBazelTokenRequest: %v", err)
	}
	if req.GetName() != "rbe-ci-token" {
		t.Fatalf("token name = %q, want rbe-ci-token", req.GetName())
	}
	if req.GetScope() != iamv1beta.RevokableToken_TENANT_MEMBERSHIP_SCOPE {
		t.Fatalf("token scope = %v, want user scope", req.GetScope())
	}

	grants := req.GetAccess().GetGrants()
	if len(grants) != 2 {
		t.Fatalf("grant count = %d, want 2", len(grants))
	}
	assertGrant := func(got *iamv1beta.Permission, resourceType, action string) {
		t.Helper()
		if got.GetResourceType() != resourceType || got.GetResourceId() != "*" || len(got.GetActions()) != 1 || got.GetActions()[0] != action {
			t.Fatalf("unexpected grant: %v", got)
		}
	}
	assertGrant(grants[0], "bazel/execution", "ensure")
	assertGrant(grants[1], "ingress", "access")

	tenantReq, err := newBazelTokenRequest("rbe-ci-token", expiresAt, "tenant")
	if err != nil {
		t.Fatalf("newBazelTokenRequest tenant scope: %v", err)
	}
	if tenantReq.GetScope() != iamv1beta.RevokableToken_TENANT_SCOPE {
		t.Fatalf("token scope = %v, want tenant scope", tenantReq.GetScope())
	}

	if _, err := newBazelTokenRequest("rbe-ci-token", expiresAt, "invalid"); err == nil {
		t.Fatal("expected invalid token scope error")
	}
}

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

func TestWriteBazelInvocationReportRecord(t *testing.T) {
	t.Parallel()

	record := &bazelv1beta.StreamInvocationReportResponse{
		BuildToolLogs: []*bazelv1beta.InvocationReportBuildToolLog{{
			Name: "stdout",
			Text: "build complete",
		}},
	}
	var output bytes.Buffer
	if err := writeBazelInvocationReportRecord(&output, record); err != nil {
		t.Fatalf("writeBazelInvocationReportRecord: %v", err)
	}

	got := output.String()
	if !strings.HasSuffix(got, "\n") {
		t.Fatalf("record is not newline terminated: %q", got)
	}
	if !strings.Contains(got, `"build_tool_logs"`) {
		t.Fatalf("record does not use protobuf field names: %q", got)
	}
	if strings.Contains(got, `"buildToolLogs"`) {
		t.Fatalf("record uses lowerCamelCase JSON field names: %q", got)
	}
}

func TestToBazelConfigExperimentalDirect(t *testing.T) {
	t.Parallel()

	config, err := toBazelConfig(context.Background(), bazelSetup{
		Endpoint:     "grpcs://cache.example:444",
		ServerCaCert: "/tmp/server_ca.cert",
		ClientCert:   "/tmp/client.cert",
		ClientKey:    "/tmp/client.key",
	}, false, "build", false)
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

func TestToBazelConfigBuildEventsDisabled(t *testing.T) {
	t.Parallel()

	config, err := toBazelConfig(context.Background(), bazelSetup{
		Endpoint:           "grpcs://cache.example:444",
		BuildEventEndpoint: "grpcs://api.us-east1.namespaceapis.com",
		StaticToken:        "tok123",
	}, false, "build", true)
	if err != nil {
		t.Fatalf("toBazelConfig: %v", err)
	}

	got := string(config)
	if strings.Contains(got, "--bes_") {
		t.Fatalf("disabled build event config contains BES fields: %q", got)
	}
	for _, want := range []string{
		"build --remote_cache=grpcs://cache.example:444\n",
		"build --remote_header=x-nsc-ingress-auth=Bearer\\ tok123\n",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing config line %q in %q", want, got)
		}
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
