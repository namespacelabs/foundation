// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package buildkite

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/std/tasks"
	buildkitepb "namespacelabs.dev/integrations/proto/namespace/cloud/buildkite"
	"namespacelabs.dev/integrations/proto/namespace/cloud/buildkite/buildkiteconnect"
	iamv1beta "namespacelabs.dev/integrations/proto/namespace/cloud/iam/v1beta"
)

type fakeQueueServiceClient struct {
	listed          bool
	getQueueID      string
	currentSettings *buildkitepb.QueueSettings
	updateRequest   *buildkitepb.UpdateQueueRequest
	calls           []string
}

func (f *fakeQueueServiceClient) ListQueues(context.Context, *connect.Request[buildkitepb.ListQueuesRequest]) (*connect.Response[buildkitepb.ListQueuesResponse], error) {
	f.listed = true
	return connect.NewResponse(&buildkitepb.ListQueuesResponse{
		Queues: []*buildkitepb.BuildkiteQueue{{Id: "queue-1", QueueName: "default"}},
	}), nil
}

func (f *fakeQueueServiceClient) GetQueue(_ context.Context, request *connect.Request[buildkitepb.GetQueueRequest]) (*connect.Response[buildkitepb.QueueResponse], error) {
	f.calls = append(f.calls, "get")
	f.getQueueID = request.Msg.GetQueueId()
	return connect.NewResponse(&buildkitepb.QueueResponse{
		Queue: &buildkitepb.BuildkiteQueue{Id: "queue-1", QueueName: "default", Settings: f.currentSettings},
	}), nil
}

func (f *fakeQueueServiceClient) UpdateQueue(_ context.Context, request *connect.Request[buildkitepb.UpdateQueueRequest]) (*connect.Response[buildkitepb.QueueResponse], error) {
	f.calls = append(f.calls, "update")
	f.updateRequest = request.Msg
	return connect.NewResponse(&buildkitepb.QueueResponse{
		Queue: &buildkitepb.BuildkiteQueue{
			Id:       "queue-1",
			Settings: &buildkitepb.QueueSettings{Permissions: &buildkitepb.Permissions{PermissionsType: buildkitepb.PermissionsType_CUSTOM}},
		},
	}), nil
}

func TestQueuesList(t *testing.T) {
	for _, name := range []string{"queue", "queues"} {
		t.Run(name, func(t *testing.T) {
			fake := installFakeClient(t)
			stdout, err := runBuildkiteCommand(t, name, "list", "-o", "json")
			if err != nil {
				t.Fatalf("command failed: %v", err)
			}
			output := &buildkitepb.ListQueuesResponse{}
			if err := protojson.Unmarshal(stdout, output); err != nil {
				t.Fatalf("decode output: %v", err)
			}
			if !fake.listed || len(output.GetQueues()) != 1 || output.GetQueues()[0].GetQueueName() != "default" {
				t.Fatalf("listed = %v, output = %#v", fake.listed, output)
			}
		})
	}
}

func TestQueuesAliasHidden(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"queue", "--help"}, {"__complete", "queue"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			cmd := NewBuildkiteCmd()
			var output bytes.Buffer
			cmd.SetOut(&output)
			cmd.SetErr(&output)
			cmd.SetArgs(args)
			if err := cmd.Execute(); err != nil {
				t.Fatalf("command failed: %v", err)
			}
			if !strings.Contains(output.String(), "queue") {
				t.Fatalf("output missing queue command: %s", &output)
			}
			for _, line := range strings.Split(output.String(), "\n") {
				fields := strings.Fields(line)
				if len(fields) > 0 && fields[0] == "queues" {
					t.Fatalf("output advertises hidden alias: %s", &output)
				}
			}
		})
	}
}

func TestQueuesGetAliasHidden(t *testing.T) {
	cmd := NewBuildkiteCmd()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs([]string{"queue", "--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if !strings.Contains(output.String(), "describe") {
		t.Fatalf("output missing describe command: %s", &output)
	}
	for _, line := range strings.Split(output.String(), "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 && fields[0] == "get" {
			t.Fatalf("output advertises hidden get alias: %s", &output)
		}
	}
}

func TestQueuesDescribeJSON(t *testing.T) {
	for _, name := range []string{"describe", "get"} {
		t.Run(name, func(t *testing.T) {
			fake := installFakeClient(t)
			fake.currentSettings.EgressPolicyTag = "restricted"
			stdout, err := runBuildkiteCommand(t, "queue", name, "queue-1", "--output", "json")
			if err != nil {
				t.Fatalf("command failed: %v", err)
			}
			output := &buildkitepb.QueueResponse{}
			if err := protojson.Unmarshal(stdout, output); err != nil {
				t.Fatalf("decode output: %v", err)
			}
			if fake.getQueueID != "queue-1" || output.GetQueue().GetId() != "queue-1" || output.GetQueue().GetSettings().GetEgressPolicyTag() != "restricted" {
				t.Fatalf("queue ID = %q, output = %#v", fake.getQueueID, output)
			}
		})
	}
}

func TestQueuesDescribePlain(t *testing.T) {
	fake := installFakeClient(t)
	fake.currentSettings.EgressPolicyTag = "restricted"
	fake.currentSettings.Permissions = &buildkitepb.Permissions{
		PermissionsType: buildkitepb.PermissionsType_CUSTOM,
		WorkloadPermissions: []*iamv1beta.Permission{
			{ResourceType: "vault/object", ResourceId: "secret-1", Actions: []string{"read", "list"}},
		},
	}
	stdout, err := runBuildkiteCommand(t, "queue", "describe", "queue-1")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	for _, want := range []string{
		"Queue ID:      queue-1",
		"Name:          default",
		"Permissions:   CUSTOM",
		"Egress Policy: restricted",
		"Resource: vault/object secret-1",
		"Actions:  read, list",
	} {
		if !strings.Contains(string(stdout), want) {
			t.Errorf("output missing %q:\n%s", want, stdout)
		}
	}
}

func TestQueuesDescribePlainOmitsUnsetSettings(t *testing.T) {
	fake := installFakeClient(t)
	fake.currentSettings = nil
	stdout, err := runBuildkiteCommand(t, "queue", "describe", "queue-1")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	for _, unwanted := range []string{"Settings:", "Permissions:", "Egress Policy:"} {
		if strings.Contains(string(stdout), unwanted) {
			t.Errorf("output contains %q:\n%s", unwanted, stdout)
		}
	}
}

func TestQueuesDescribeSpecRoundTripsThroughUpdate(t *testing.T) {
	fake := installFakeClient(t)
	fake.currentSettings.EgressPolicyTag = "restricted"
	fake.currentSettings.Permissions = &buildkitepb.Permissions{
		PermissionsType: buildkitepb.PermissionsType_CUSTOM,
		WorkloadPermissions: []*iamv1beta.Permission{
			{ResourceType: "vault/object", ResourceId: "secret-1", Actions: []string{"read"}},
		},
	}
	stdout, err := runBuildkiteCommand(t, "queue", "describe", "queue-1", "-o", "spec")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if strings.Contains(string(stdout), "queue-1") || !strings.Contains(string(stdout), "egress_policy_tag") {
		t.Fatalf("spec output = %s, want only settings with proto field names", stdout)
	}

	specPath := filepath.Join(t.TempDir(), "spec.json")
	if err := os.WriteFile(specPath, stdout, 0o600); err != nil {
		t.Fatalf("write spec file: %v", err)
	}
	if _, err := runBuildkiteCommand(t, "queue", "update", "queue-1", "--spec_file", specPath); err != nil {
		t.Fatalf("update failed: %v", err)
	}
	if !proto.Equal(fake.updateRequest.GetSettings(), fake.currentSettings) {
		t.Fatalf("updated settings = %v, want %v", fake.updateRequest.GetSettings(), fake.currentSettings)
	}
}

func TestQueuesDescribeSpecEmptySettings(t *testing.T) {
	fake := installFakeClient(t)
	fake.currentSettings = nil
	stdout, err := runBuildkiteCommand(t, "queue", "describe", "queue-1", "-o", "spec")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if got := strings.TrimSpace(string(stdout)); got != "{}" {
		t.Fatalf("spec output = %q, want {}", got)
	}
}

func TestQueuesInvalidOutput(t *testing.T) {
	installFakeClient(t)
	for _, args := range [][]string{{"list", "-o", "spec"}, {"describe", "queue-1", "-o", "yaml"}, {"update", "queue-1", "--reset", "-o", "spec"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			_, err := runBuildkiteCommand(t, append([]string{"queue"}, args...)...)
			if err == nil || !strings.Contains(err.Error(), "invalid --output") {
				t.Fatalf("error = %v, want invalid output error", err)
			}
		})
	}
}

func TestQueuesUpdateCustomPermissions(t *testing.T) {
	fake := installFakeClient(t)
	fake.currentSettings.EgressPolicyTag = "existing-policy"
	unknownSetting := protowire.AppendString(protowire.AppendTag(nil, 100, protowire.BytesType), "future-setting")
	existingSettings := append([]byte(nil), fake.currentSettings.ProtoReflect().GetUnknown()...)
	existingSettings = append(existingSettings, unknownSetting...)
	fake.currentSettings.ProtoReflect().SetUnknown(existingSettings)
	stdout, err := runBuildkiteCommand(t,
		"queue", "update", "queue-1",
		"--workload_permissions", `{"resource_type":"vault/object","resource_id":"secret-1","actions":["read"]}`,
		"-o", "json",
	)
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}

	req := fake.updateRequest
	if req == nil || req.GetQueueId() != "queue-1" {
		t.Fatalf("request = %#v, want queue-1", req)
	}
	if got := strings.Join(fake.calls, ","); got != "get,update" {
		t.Fatalf("calls = %q, want get before update", got)
	}
	if got := string(req.GetSettings().ProtoReflect().GetUnknown()); got != string(existingSettings) {
		t.Fatalf("unknown settings = %q, want fetched settings preserved", got)
	}
	if got := req.GetSettings().GetEgressPolicyTag(); got != "existing-policy" {
		t.Fatalf("egress policy = %q, want fetched egress policy preserved", got)
	}
	permissions := req.GetSettings().GetPermissions()
	if permissions.GetPermissionsType() != buildkitepb.PermissionsType_CUSTOM || len(permissions.GetWorkloadPermissions()) != 1 {
		t.Fatalf("permissions = %#v, want one custom grant", permissions)
	}
	grant := permissions.GetWorkloadPermissions()[0]
	if grant.GetResourceType() != "vault/object" || grant.GetResourceId() != "secret-1" || len(grant.GetActions()) != 1 || grant.GetActions()[0] != "read" {
		t.Fatalf("grant = %#v, want parsed grant", grant)
	}
	output := &buildkitepb.QueueResponse{}
	if err := protojson.Unmarshal(stdout, output); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if output.GetQueue().GetSettings().GetPermissions().GetPermissionsType() != buildkitepb.PermissionsType_CUSTOM {
		t.Fatalf("output = %#v, want updated settings", output)
	}
}

func TestQueuesUpdatePlainOutput(t *testing.T) {
	installFakeClient(t)
	stdout, err := runBuildkiteCommand(t, "queue", "update", "queue-1", "--egress_policy", "restricted")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	for _, want := range []string{"Queue updated successfully:", "Queue ID:      queue-1", "Permissions:   CUSTOM"} {
		if !strings.Contains(string(stdout), want) {
			t.Errorf("output missing %q:\n%s", want, stdout)
		}
	}
}

func TestQueuesUpdateReset(t *testing.T) {
	fake := installFakeClient(t)
	if _, err := runBuildkiteCommand(t, "queues", "update", "queue-1", "--reset"); err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if got := strings.Join(fake.calls, ","); got != "get,update" {
		t.Fatalf("calls = %q, want get before update", got)
	}
	if fake.updateRequest == nil || fake.updateRequest.GetSettings() == nil || fake.updateRequest.GetSettings().GetPermissions() != nil {
		t.Fatalf("request = %#v, want explicit empty settings", fake.updateRequest)
	}
}

func TestQueuesUpdateSpecFileReplacesSettingsWithoutFetch(t *testing.T) {
	fake := installFakeClient(t)
	fake.currentSettings.EgressPolicyTag = "old-policy"
	fake.currentSettings.ProtoReflect().SetUnknown(append(
		fake.currentSettings.ProtoReflect().GetUnknown(),
		protowire.AppendString(protowire.AppendTag(nil, 100, protowire.BytesType), "future-setting")...,
	))
	specPath := filepath.Join(t.TempDir(), "queue-settings.json")
	spec := `{
  "permissions": {
    "permissions_type": "CUSTOM",
    "workload_permissions": [{"resource_type":"vault/object","resource_id":"secret-1","actions":["read"]}]
  },
  "egress_policy_tag": "restricted"
}`
	if err := os.WriteFile(specPath, []byte(spec), 0o600); err != nil {
		t.Fatalf("write spec file: %v", err)
	}

	if _, err := runBuildkiteCommand(t, "queues", "update", "queue-1", "--spec_file", specPath); err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if got := strings.Join(fake.calls, ","); got != "update" {
		t.Fatalf("calls = %q, want update without get", got)
	}
	settings := fake.updateRequest.GetSettings()
	if settings.GetPermissions().GetPermissionsType() != buildkitepb.PermissionsType_CUSTOM || len(settings.GetPermissions().GetWorkloadPermissions()) != 1 {
		t.Fatalf("permissions = %#v, want complete spec permissions", settings.GetPermissions())
	}
	if got := settings.GetEgressPolicyTag(); got != "restricted" {
		t.Fatalf("egress policy = %q, want restricted", got)
	}
	unknown := settings.ProtoReflect().GetUnknown()
	if strings.Contains(string(unknown), "future-setting") {
		t.Fatalf("settings preserved fetched field during complete replacement: %q", unknown)
	}
}

func TestQueuesUpdateEgressPolicyPreservesPermissions(t *testing.T) {
	fake := installFakeClient(t)
	fake.currentSettings.EgressPolicyTag = "old-policy"

	if _, err := runBuildkiteCommand(t, "queues", "update", "queue-1", "--egress_policy", "restricted"); err != nil {
		t.Fatalf("command failed: %v", err)
	}
	settings := fake.updateRequest.GetSettings()
	if got := strings.Join(fake.calls, ","); got != "get,update" {
		t.Fatalf("calls = %q, want get before update", got)
	}
	if settings.GetPermissions().GetPermissionsType() != buildkitepb.PermissionsType_DEFAULT {
		t.Fatalf("permissions = %#v, want fetched permissions preserved", settings.GetPermissions())
	}
	if got := settings.GetEgressPolicyTag(); got != "restricted" {
		t.Fatalf("egress policy = %q, want restricted", got)
	}
}

func TestQueuesRemoveEgressPolicyPreservesPermissions(t *testing.T) {
	fake := installFakeClient(t)
	fake.currentSettings.EgressPolicyTag = "restricted"

	if _, err := runBuildkiteCommand(t, "queues", "update", "queue-1", "--remove_egress_policy"); err != nil {
		t.Fatalf("command failed: %v", err)
	}
	settings := fake.updateRequest.GetSettings()
	if settings.GetPermissions().GetPermissionsType() != buildkitepb.PermissionsType_DEFAULT {
		t.Fatalf("permissions = %#v, want fetched permissions preserved", settings.GetPermissions())
	}
	if got := settings.GetEgressPolicyTag(); got != "" {
		t.Fatalf("egress policy = %q, want removed", got)
	}
}

func TestQueuesResetPermissionsPreservesEgressPolicy(t *testing.T) {
	fake := installFakeClient(t)
	fake.currentSettings.Permissions = &buildkitepb.Permissions{PermissionsType: buildkitepb.PermissionsType_CUSTOM}
	fake.currentSettings.EgressPolicyTag = "restricted"

	if _, err := runBuildkiteCommand(t, "queues", "update", "queue-1", "--reset_permissions"); err != nil {
		t.Fatalf("command failed: %v", err)
	}
	settings := fake.updateRequest.GetSettings()
	if settings.GetPermissions().GetPermissionsType() != buildkitepb.PermissionsType_DEFAULT {
		t.Fatalf("permissions = %#v, want default permissions", settings.GetPermissions())
	}
	if got := settings.GetEgressPolicyTag(); got != "restricted" {
		t.Fatalf("egress policy = %q, want fetched egress policy preserved", got)
	}
}

func TestQueuesUpdateRequiresMode(t *testing.T) {
	installFakeClient(t)
	_, err := runBuildkiteCommand(t, "queues", "update", "queue-1")
	if err == nil || !strings.Contains(err.Error(), "--workload_permissions, --reset_permissions, --egress_policy, --remove_egress_policy, or --reset is required") {
		t.Fatalf("error = %v, want required mode error", err)
	}
}

func installFakeClient(t *testing.T) *fakeQueueServiceClient {
	t.Helper()
	fake := &fakeQueueServiceClient{
		currentSettings: &buildkitepb.QueueSettings{
			Permissions: &buildkitepb.Permissions{PermissionsType: buildkitepb.PermissionsType_DEFAULT},
		},
	}
	original := newQueueServiceClient
	newQueueServiceClient = func(context.Context) (buildkiteconnect.QueueServiceClient, error) { return fake, nil }
	t.Cleanup(func() { newQueueServiceClient = original })
	return fake
}

func runBuildkiteCommand(t *testing.T, args ...string) ([]byte, error) {
	t.Helper()
	ctx := tasks.WithSink(context.Background(), tasks.NullSink())
	cmd := NewBuildkiteCmd()
	cmd.SetArgs(args)

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = oldStdout }()

	runErr := tasks.Action("test.buildkite").Run(ctx, func(ctx context.Context) error {
		return cmd.ExecuteContext(ctx)
	})
	if err := w.Close(); err != nil {
		t.Fatalf("close stdout: %v", err)
	}
	stdout, readErr := io.ReadAll(r)
	if readErr != nil {
		t.Fatalf("read stdout: %v", readErr)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("close stdout reader: %v", err)
	}
	return stdout, runErr
}
