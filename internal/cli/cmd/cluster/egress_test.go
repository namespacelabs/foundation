// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"connectrpc.com/connect"
	networkv1beta "namespacelabs.dev/integrations/proto/namespace/cloud/network/v1beta"
	"namespacelabs.dev/integrations/proto/namespace/cloud/network/v1beta/networkv1betaconnect"
)

func TestEgressPolicyUpdateUsesCurrentRevision(t *testing.T) {
	specFile := filepath.Join(t.TempDir(), "policy.json")
	if err := os.WriteFile(specFile, []byte(`{
  "description": "updated",
  "mode": "BLOCK"
}`), 0o600); err != nil {
		t.Fatal(err)
	}

	client := &fakeEgressPolicyClient{t: t}
	originalClient := newEgressPolicyClient
	newEgressPolicyClient = func(context.Context) (networkv1betaconnect.EgressPolicyServiceClient, error) {
		return client, nil
	}
	t.Cleanup(func() { newEgressPolicyClient = originalClient })

	cmd := newEgressPolicyUpdateCmd()
	cmd.SetArgs([]string{"example-policy", "--spec_file", specFile})
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}

	if !client.updated {
		t.Fatal("UpdateEgressPolicy was not called")
	}
}

func TestEgressPolicyUpdateRejectsUnknownCurrentFields(t *testing.T) {
	specFile := filepath.Join(t.TempDir(), "policy.json")
	if err := os.WriteFile(specFile, []byte(`{"mode":"BLOCK"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	inject := &networkv1beta.EgressPolicySpec_InjectOpts{HeaderName: "Authorization", Value: "token"}
	inject.ProtoReflect().SetUnknown([]byte{0xa0, 0x06, 0x01})
	client := &fakeEgressPolicyClient{
		t: t,
		currentPolicy: &networkv1beta.EgressPolicy{
			Tag: "example-policy",
			Spec: &networkv1beta.EgressPolicySpec{
				Rules: []*networkv1beta.EgressPolicySpec_Rule{{Inject: inject}},
			},
		},
	}
	originalClient := newEgressPolicyClient
	newEgressPolicyClient = func(context.Context) (networkv1betaconnect.EgressPolicyServiceClient, error) {
		return client, nil
	}
	t.Cleanup(func() { newEgressPolicyClient = originalClient })

	cmd := newEgressPolicyUpdateCmd()
	cmd.SetArgs([]string{"example-policy", "--spec_file", specFile})
	err := cmd.ExecuteContext(context.Background())
	if err == nil || !strings.Contains(err.Error(), "upgrade nsc") {
		t.Fatalf("error = %v, want upgrade nsc diagnostic", err)
	}
	if client.updated {
		t.Fatal("UpdateEgressPolicy was called despite unknown current fields")
	}
}

func TestEgressPolicyDescribeRejectsUnknownFields(t *testing.T) {
	policy := &networkv1beta.EgressPolicy{Tag: "example-policy"}
	policy.ProtoReflect().SetUnknown([]byte{0xa0, 0x06, 0x01})
	client := &fakeEgressPolicyClient{t: t, currentPolicy: policy}
	originalClient := newEgressPolicyClient
	newEgressPolicyClient = func(context.Context) (networkv1betaconnect.EgressPolicyServiceClient, error) {
		return client, nil
	}
	t.Cleanup(func() { newEgressPolicyClient = originalClient })

	cmd := newEgressPolicyDescribeCmd()
	cmd.SetArgs([]string{"example-policy", "--output", "json"})
	err := cmd.ExecuteContext(context.Background())
	if err == nil || !strings.Contains(err.Error(), "upgrade nsc") {
		t.Fatalf("error = %v, want upgrade nsc diagnostic", err)
	}
}

func TestPrintEgressPolicyDescription(t *testing.T) {
	policy := &networkv1beta.EgressPolicy{
		Tag:         "example-policy",
		Description: "Example policy",
		Spec: &networkv1beta.EgressPolicySpec{
			Mode:                 networkv1beta.EgressPolicySpec_BLOCK,
			DeepPacketInspection: true,
			Rules: []*networkv1beta.EgressPolicySpec_Rule{
				{Op: networkv1beta.EgressPolicySpec_Rule_ALLOW},
				{Op: networkv1beta.EgressPolicySpec_Rule_INJECT},
			},
		},
	}

	var output bytes.Buffer
	if err := printEgressPolicyDescription(&output, policy, 42); err != nil {
		t.Fatal(err)
	}
	want := "Tag:                      example-policy\n" +
		"Description:              Example policy\n" +
		"Mode:                     BLOCK\n" +
		"Deep packet inspection:   true\n" +
		"Rules:                    2\n" +
		"Revision:                 42\n"
	if output.String() != want {
		t.Fatalf("output:\n%s\nwant:\n%s", output.String(), want)
	}
}

func TestParseEgressPolicyAcceptsBothFormats(t *testing.T) {
	for _, contents := range []string{
		`{"tag":"example-policy","mode":"BLOCK"}`,
		`{"tag":"example-policy","spec":{"mode":"BLOCK"}}`,
	} {
		policy, err := parseEgressPolicy([]byte(contents))
		if err != nil {
			t.Fatalf("parseEgressPolicy(%s): %v", contents, err)
		}
		if policy.Spec == nil || policy.Spec.Mode != networkv1beta.EgressPolicySpec_BLOCK {
			t.Errorf("parseEgressPolicy(%s) spec = %v, want BLOCK mode", contents, policy.Spec)
		}
	}
}

func TestEgressPolicyPreservesDynamicEndpointResolution(t *testing.T) {
	contents := []byte(`{
  "tag": "dynamic-token",
  "mode": "BLOCK",
  "rules": [{
    "op": "INJECT",
    "inject": {
      "header_name": "Authorization",
      "from_endpoint": {
        "url": "https://credentials.example/token",
        "headers": [{"name": "X-Client-ID", "value": "client-123"}]
      },
      "resolution_policy": "RESOLUTION_POLICY_WHEN_STALE"
    }
  }]
}`)

	policy, err := parseEgressPolicy(contents)
	if err != nil {
		t.Fatal(err)
	}
	inject := policy.GetSpec().GetRules()[0].GetInject()
	if inject.GetFromEndpoint().GetUrl() != "https://credentials.example/token" {
		t.Fatalf("endpoint URL = %q", inject.GetFromEndpoint().GetUrl())
	}
	if inject.GetResolutionPolicy() != networkv1beta.EgressPolicySpec_InjectOpts_RESOLUTION_POLICY_WHEN_STALE {
		t.Fatalf("resolution policy = %s", inject.GetResolutionPolicy())
	}
	if headers := inject.GetFromEndpoint().GetHeaders(); len(headers) != 1 || headers[0].GetName() != "X-Client-ID" || headers[0].GetValue() != "client-123" {
		t.Fatalf("endpoint headers = %v", headers)
	}

	encoded, err := marshalEgressPolicy(policy)
	if err != nil {
		t.Fatal(err)
	}
	reparsed, err := parseEgressPolicy(encoded)
	if err != nil {
		t.Fatal(err)
	}
	reparsedInject := reparsed.GetSpec().GetRules()[0].GetInject()
	if reparsedInject.GetFromEndpoint().GetUrl() != inject.GetFromEndpoint().GetUrl() || reparsedInject.GetResolutionPolicy() != inject.GetResolutionPolicy() {
		t.Fatalf("dynamic endpoint resolution did not round-trip: %s", encoded)
	}
}

func TestMarshalEgressPolicyUsesFlattenedFormat(t *testing.T) {
	policy, err := parseEgressPolicy([]byte(exampleEgressPolicyJSON))
	if err != nil {
		t.Fatal(err)
	}

	encoded, err := marshalEgressPolicy(policy)
	if err != nil {
		t.Fatal(err)
	}

	var fields map[string]any
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatal(err)
	}
	if _, ok := fields["spec"]; ok {
		t.Errorf("marshalEgressPolicy() = %s, want no nested spec", encoded)
	}
	if fields["mode"] != "BLOCK" {
		t.Errorf("marshalEgressPolicy() mode = %v, want BLOCK", fields["mode"])
	}

	tag := bytes.Index(encoded, []byte(`"tag"`))
	description := bytes.Index(encoded, []byte(`"description"`))
	mode := bytes.Index(encoded, []byte(`"mode"`))
	rules := bytes.Index(encoded, []byte(`"rules"`))
	if !(tag < description && description < mode && mode < rules) {
		t.Errorf("marshalEgressPolicy() fields are not in the expected order:\n%s", encoded)
	}
}

func TestMarshalEgressPolicyRejectsNestedUnknownFields(t *testing.T) {
	endpoint := &networkv1beta.EgressPolicySpec_InjectOpts_FromEndpoint{Url: "https://credentials.example/token"}
	endpoint.ProtoReflect().SetUnknown([]byte{0xa0, 0x06, 0x01})
	policy := &networkv1beta.EgressPolicy{
		Tag: "dynamic-token",
		Spec: &networkv1beta.EgressPolicySpec{
			Rules: []*networkv1beta.EgressPolicySpec_Rule{{
				Inject: &networkv1beta.EgressPolicySpec_InjectOpts{FromEndpoint: endpoint},
			}},
		},
	}

	_, err := marshalEgressPolicy(policy)
	if err == nil || !strings.Contains(err.Error(), "upgrade nsc") {
		t.Fatalf("error = %v, want upgrade nsc diagnostic", err)
	}
}

type fakeEgressPolicyClient struct {
	t             *testing.T
	updated       bool
	currentPolicy *networkv1beta.EgressPolicy
}

func (f *fakeEgressPolicyClient) CreateEgressPolicy(context.Context, *connect.Request[networkv1beta.CreateEgressPolicyRequest]) (*connect.Response[networkv1beta.CreateEgressPolicyResponse], error) {
	f.t.Fatal("unexpected CreateEgressPolicy call")
	return nil, nil
}

func (f *fakeEgressPolicyClient) UpdateEgressPolicy(_ context.Context, req *connect.Request[networkv1beta.UpdateEgressPolicyRequest]) (*connect.Response[networkv1beta.UpdateEgressPolicyResponse], error) {
	if req.Msg.MatchRevision != 42 {
		f.t.Errorf("match revision = %d, want 42", req.Msg.MatchRevision)
	}
	if req.Msg.Policy.Tag != "example-policy" {
		f.t.Errorf("policy tag = %q, want example-policy", req.Msg.Policy.Tag)
	}
	if req.Msg.Policy.Spec == nil || req.Msg.Policy.Spec.Mode != networkv1beta.EgressPolicySpec_BLOCK {
		f.t.Errorf("policy spec = %v, want BLOCK mode", req.Msg.Policy.Spec)
	}
	f.updated = true
	return connect.NewResponse(&networkv1beta.UpdateEgressPolicyResponse{Revision: 43}), nil
}

func (f *fakeEgressPolicyClient) GetEgressPolicy(_ context.Context, req *connect.Request[networkv1beta.GetEgressPolicyRequest]) (*connect.Response[networkv1beta.GetEgressPolicyResponse], error) {
	if req.Msg.Tag != "example-policy" {
		f.t.Errorf("get policy tag = %q, want example-policy", req.Msg.Tag)
	}
	policy := f.currentPolicy
	if policy == nil {
		policy = &networkv1beta.EgressPolicy{Tag: req.Msg.Tag}
	}
	return connect.NewResponse(&networkv1beta.GetEgressPolicyResponse{
		Policy:   policy,
		Revision: 42,
	}), nil
}

func (f *fakeEgressPolicyClient) ListEgressPolicies(context.Context, *connect.Request[networkv1beta.ListEgressPoliciesRequest]) (*connect.Response[networkv1beta.ListEgressPoliciesResponse], error) {
	f.t.Fatal("unexpected ListEgressPolicies call")
	return nil, nil
}
