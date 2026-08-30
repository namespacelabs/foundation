// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package integrations

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"

	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/std/tasks"
)

func TestTailscaleListCmdJSON(t *testing.T) {
	original := getTenantIntegrations
	t.Cleanup(func() { getTenantIntegrations = original })

	want := fnapi.GetTenantIntegrationsResponse{
		MetadataVersion: 7,
		Tailscale: map[string]fnapi.TailscaleSpec{
			"build": {
				OauthClientId: "client-build",
				Tags:          []string{"tag:ci", "tag:dev"},
			},
			"runner": {
				OauthClientId: "client-runner",
			},
		},
	}

	getTenantIntegrations = func(context.Context) (fnapi.GetTenantIntegrationsResponse, error) {
		return want, nil
	}

	stdout, err := runIntegrationsCommand(t, "tailscale", "list", "--output", "json")
	if err != nil {
		t.Fatalf("runIntegrationsCommand() error = %v", err)
	}

	var got fnapi.GetTenantIntegrationsResponse
	if err := json.Unmarshal(stdout, &got); err != nil {
		t.Fatalf("json.Unmarshal(stdout) error = %v, stdout = %q", err, stdout)
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("list json output = %#v, want %#v", got, want)
	}
}

func TestTailscaleListCmdPlain(t *testing.T) {
	original := getTenantIntegrations
	t.Cleanup(func() { getTenantIntegrations = original })

	getTenantIntegrations = func(context.Context) (fnapi.GetTenantIntegrationsResponse, error) {
		return fnapi.GetTenantIntegrationsResponse{
			Tailscale: map[string]fnapi.TailscaleSpec{
				"zebra": {
					OauthClientId: "client-z",
				},
				"alpha": {
					OauthClientId: "client-a",
					Tags:          []string{"tag:one", "tag:two"},
				},
			},
		}, nil
	}

	stdout, err := runIntegrationsCommand(t, "tailscale", "list")
	if err != nil {
		t.Fatalf("runIntegrationsCommand() error = %v", err)
	}

	got := string(stdout)
	if !strings.Contains(got, "Tailscale integrations:\n\nalpha\n  OAuth Client ID: client-a\n  Tags: tag:one, tag:two\n\nzebra\n  OAuth Client ID: client-z\n  Tags: none\n") {
		t.Fatalf("plain list output = %q, want sorted integrations with tags", got)
	}

	if strings.Index(got, "alpha") > strings.Index(got, "zebra") {
		t.Fatalf("plain list output = %q, want alpha before zebra", got)
	}
}

func runIntegrationsCommand(t *testing.T, args ...string) ([]byte, error) {
	t.Helper()

	ctx := tasks.WithSink(context.Background(), tasks.NullSink())
	actionID := tasks.ActionID("test.integrations")

	cmd := NewIntegrationsCmd()
	cmd.SetArgs(args)

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	os.Stdout = w
	defer func() {
		os.Stdout = oldStdout
	}()

	runErr := tasks.Action("test.integrations").ID(actionID).Run(ctx, func(ctx context.Context) error {
		return cmd.ExecuteContext(ctx)
	})

	if err := w.Close(); err != nil {
		t.Fatalf("stdout.Close() error = %v", err)
	}

	data, readErr := io.ReadAll(r)
	if readErr != nil {
		t.Fatalf("io.ReadAll(stdout) error = %v", readErr)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("reader.Close() error = %v", err)
	}

	return data, runErr
}
