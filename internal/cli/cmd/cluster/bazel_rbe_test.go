// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"strings"
	"testing"
)

func TestToBazelExecutionConfigBuildEventsStatic(t *testing.T) {
	t.Parallel()

	config, err := toBazelExecutionConfig(context.Background(), bazelRbeSetup{
		SchedulerEndpoint:  "grpcs://scheduler.example:443",
		StorageEndpoint:    "grpcs://storage.example:443",
		IngressAuthToken:   "tok123",
		BuildEventEndpoint: "grpcs://api.us-east1.namespaceapis.com",
	}, "build")
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
	}, "build")
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
	}, "build")
	if err != nil {
		t.Fatalf("toBazelExecutionConfig: %v", err)
	}

	got := string(config)
	if strings.Contains(got, "--bes_backend") || strings.Contains(got, "credential_helper") {
		t.Fatalf("must not configure build events when no endpoint is returned: %q", got)
	}
}
