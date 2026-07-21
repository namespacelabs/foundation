// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestToBazelExecutionConfigBuildEventsStatic(t *testing.T) {
	t.Parallel()

	config, err := toBazelExecutionConfig(context.Background(), bazelRbeSetup{
		SchedulerEndpoint:  "grpcs://scheduler.example:443",
		StorageEndpoint:    "grpcs://storage.example:443",
		IngressAuthToken:   "tok123",
		BuildEventEndpoint: "grpcs://api.us-east1.namespaceapis.com",
	}, "build", true, false)
	if err != nil {
		t.Fatalf("toBazelExecutionConfig: %v", err)
	}

	got := string(config)
	for _, want := range []string{
		"build --bes_backend=grpcs://api.us-east1.namespaceapis.com\n",
		"build --bes_header=Authorization=Bearer\\ tok123\n",
		"build --bes_header=x-nsc-ingress-auth=Bearer\\ tok123\n",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing config line %q in %q", want, got)
		}
	}

	if strings.Contains(got, "credential_helper") {
		t.Fatalf("static mode must not configure the credential helper: %q", got)
	}
}

func TestToBazelExecutionConfigBuildEventsMTLS(t *testing.T) {
	t.Parallel()

	config, err := toBazelExecutionConfig(context.Background(), bazelRbeSetup{
		SchedulerEndpoint:       "grpcs://scheduler.example:444",
		StorageEndpoint:         "grpcs://storage.example:444",
		ClientCert:              "/tmp/client.cert",
		ClientKey:               "/tmp/client.key",
		BuildEventEndpoint:      "grpcs://api.us-east1.namespaceapis.com",
		CredentialHelperDomains: []string{"api.us-east1.namespaceapis.com"},
	}, "build", true, false)
	if err != nil {
		t.Fatalf("toBazelExecutionConfig: %v", err)
	}

	got := string(config)
	for _, want := range []string{
		"build --bes_backend=grpcs://api.us-east1.namespaceapis.com\n",
		"build --credential_helper=*.api.us-east1.namespaceapis.com=" + BazelCredHelperBinary + "\n",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing config line %q in %q", want, got)
		}
	}

	if strings.Contains(got, "--bes_header=") {
		t.Fatalf("mTLS mode must not configure bes headers: %q", got)
	}
}

func TestToBazelExecutionConfigNoBuildEvents(t *testing.T) {
	t.Parallel()

	config, err := toBazelExecutionConfig(context.Background(), bazelRbeSetup{
		SchedulerEndpoint: "grpcs://scheduler.example:444",
		StorageEndpoint:   "grpcs://storage.example:444",
		ClientCert:        "/tmp/client.cert",
		ClientKey:         "/tmp/client.key",
	}, "build", true, false)
	if err != nil {
		t.Fatalf("toBazelExecutionConfig: %v", err)
	}

	got := string(config)
	if strings.Contains(got, "--bes_backend") || strings.Contains(got, "credential_helper") {
		t.Fatalf("must not configure build events when no endpoint is returned: %q", got)
	}
}

func TestToBazelExecutionConfigBuildEventsDisabled(t *testing.T) {
	t.Parallel()

	config, err := toBazelExecutionConfig(context.Background(), bazelRbeSetup{
		SchedulerEndpoint:       "grpcs://scheduler.example:444",
		StorageEndpoint:         "grpcs://storage.example:444",
		BuildEventEndpoint:      "grpcs://api.us-east1.namespaceapis.com",
		CredentialHelperDomains: []string{"api.us-east1.namespaceapis.com"},
	}, "build", true, true)
	if err != nil {
		t.Fatalf("toBazelExecutionConfig: %v", err)
	}

	got := string(config)
	for _, unwanted := range []string{"--bes_backend", "--bes_header", "credential_helper"} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("disabled build event config contains %q: %q", unwanted, got)
		}
	}
	if !strings.Contains(got, "build --remote_cache=grpcs://storage.example:444\n") {
		t.Fatalf("disabled build events removed remote cache config: %q", got)
	}
}

func TestToBazelExecutionConfigWithoutRemoteExecution(t *testing.T) {
	t.Parallel()

	config, err := toBazelExecutionConfig(context.Background(), bazelRbeSetup{
		SchedulerEndpoint:     "grpcs://scheduler.example:443",
		StorageEndpoint:       "grpcs://storage.example:443",
		IngressAuthToken:      "tok123",
		RemoteLocalFallback:   true,
		RemoteDownloadOutputs: "minimal",
		RemoteTimeout:         5 * time.Minute,
		Jobs:                  32,
		BuildEventEndpoint:    "grpcs://api.us-east1.namespaceapis.com",
	}, "build", false, false)
	if err != nil {
		t.Fatalf("toBazelExecutionConfig: %v", err)
	}

	got := string(config)
	for _, want := range []string{
		"build --remote_cache=grpcs://storage.example:443\n",
		"build --remote_header=x-nsc-ingress-auth=Bearer\\ tok123\n",
		"build --remote_download_outputs=minimal\n",
		"build --jobs=32\n",
		"build --remote_timeout=300\n",
		"build --bes_backend=grpcs://api.us-east1.namespaceapis.com\n",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing config line %q in %q", want, got)
		}
	}

	for _, unwanted := range []string{
		"--remote_executor=",
		"--spawn_strategy=remote",
		"--genrule_strategy=remote",
		"--remote_local_fallback=",
	} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("remote execution config %q present in %q", unwanted, got)
		}
	}
}

func TestNewBazelCmdSetupAlias(t *testing.T) {
	t.Parallel()

	cmd := NewBazelCmd()
	setup, _, err := cmd.Find([]string{"setup"})
	if err != nil {
		t.Fatalf("finding bazel setup: %v", err)
	}
	if !setup.Hidden {
		t.Fatal("bazel setup must be hidden")
	}

	remote := setup.Flags().Lookup("remote")
	if remote == nil {
		t.Fatal("bazel setup is missing --remote")
	}
	if remote.DefValue != "true" {
		t.Fatalf("--remote default = %q, want true", remote.DefValue)
	}
	if setup.Flags().Lookup("token") == nil {
		t.Fatal("bazel setup is missing --token")
	}
	if setup.Flags().Lookup("disable_build_events") == nil {
		t.Fatal("bazel setup is missing --disable_build_events")
	}

	executionSetup, _, err := cmd.Find([]string{"execution", "setup"})
	if err != nil {
		t.Fatalf("finding bazel execution setup: %v", err)
	}
	if executionSetup.Flags().Lookup("remote") != nil {
		t.Fatal("bazel execution setup must not expose --remote")
	}
	if executionSetup.Flags().Lookup("token") == nil {
		t.Fatal("bazel execution setup is missing --token")
	}
	if executionSetup.Flags().Lookup("disable_build_events") == nil {
		t.Fatal("bazel execution setup is missing --disable_build_events")
	}
}
