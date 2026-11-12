// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package auth

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"gotest.tools/assert"
	"namespacelabs.dev/foundation/internal/fnapi"
)

func TestTrustRelationshipMaximumDurationFlags(t *testing.T) {
	add := newTrustAddCmd()
	if flag := add.Flags().Lookup("maximum_token_duration"); flag == nil {
		t.Fatal("add is missing --maximum_token_duration")
	} else if flag.DefValue != "86400s" {
		t.Fatalf("add maximum default = %q, want 86400s", flag.DefValue)
	}

	for _, cmd := range []*cobra.Command{add, newTrustUpdateCmd()} {
		if flag := cmd.Flags().Lookup("maximum_token_duration"); flag == nil {
			t.Fatalf("%s is missing --maximum_token_duration", cmd.Name())
		}
	}
}

func TestUpdatedTrustRelationshipMaximumDuration(t *testing.T) {
	existing := fnapi.StoredTrustRelationship{MaximumTokenDuration: "3600s"}

	if got := updatedTrustRelationship(existing, trustRelationshipUpdate{}); got.MaximumTokenDuration != "3600s" {
		t.Fatalf("omitted maximum changed to %q", got.MaximumTokenDuration)
	}

	clear := ""
	if got := updatedTrustRelationship(existing, trustRelationshipUpdate{maximumTokenDuration: &clear}); got.MaximumTokenDuration != "0s" {
		t.Fatalf("cleared maximum changed to %q", got.MaximumTokenDuration)
	}

	set := "7200s"
	if got := updatedTrustRelationship(existing, trustRelationshipUpdate{maximumTokenDuration: &set}); got.MaximumTokenDuration != set {
		t.Fatalf("set maximum changed to %q", got.MaximumTokenDuration)
	}
}

func TestTrustRelationshipClaimMatches(t *testing.T) {
	const value = `{"repository":"acme/mobile","job_workflow_ref":"acme/automation/.github/workflows/job-prepare-simulator-dependencies.yaml@refs/heads/main","ref":"refs/heads/main"}`
	for _, newCommand := range []func() *cobra.Command{newTrustAddCmd, newTrustUpdateCmd} {
		cmd := newCommand()
		t.Run(cmd.Name(), func(t *testing.T) {
			assert.NilError(t, cmd.ParseFlags([]string{"--claim_matches", value}))
			raw, err := cmd.Flags().GetString("claim_matches")
			assert.NilError(t, err)
			matches, err := parseClaimMatches(raw)
			assert.NilError(t, err)
			assert.DeepEqual(t, matches, map[string]string{
				"repository":       "acme/mobile",
				"job_workflow_ref": "acme/automation/.github/workflows/job-prepare-simulator-dependencies.yaml@refs/heads/main",
				"ref":              "refs/heads/main",
			})
		})
	}
}

func TestTrustRelationshipSubjectMatchAlias(t *testing.T) {
	for _, newCommand := range []func() *cobra.Command{newTrustAddCmd, newTrustUpdateCmd} {
		for _, arg := range []string{"--subject_match", "--subject-match"} {
			cmd := newCommand()
			t.Run(cmd.Name()+arg, func(t *testing.T) {
				assert.NilError(t, cmd.ParseFlags([]string{arg, "repo:acme/*"}))
				assert.Assert(t, cmd.Flags().Changed("subject_match"))
				raw, err := cmd.Flags().GetString("subject_match")
				assert.NilError(t, err)
				assert.Equal(t, raw, "repo:acme/*")
			})
		}

		cmd := newCommand()
		assert.Assert(t, cmd.Flags().Lookup("subject-match") != nil, "alias should resolve")
		assert.Assert(t, !strings.Contains(cmd.Flags().FlagUsages(), "--subject-match"), "alias should be hidden from help")
		assert.Assert(t, strings.Contains(cmd.Flags().FlagUsages(), "--subject_match"))
	}
}

func TestParseClaimMatches(t *testing.T) {
	for _, value := range []string{`repository=acme/mobile`, `{"repository":`, `{"repository":42}`, `[]`, `null`} {
		t.Run(value, func(t *testing.T) {
			_, err := parseClaimMatches(value)
			assert.Assert(t, err != nil)
		})
	}

	matches, err := parseClaimMatches(`{}`)
	assert.NilError(t, err)
	assert.DeepEqual(t, matches, map[string]string{})
}

func TestUpdatedTrustRelationshipClaimMatches(t *testing.T) {
	replacement := map[string]string{"ref": "refs/heads/release/*"}
	empty := map[string]string{}
	for _, tc := range []struct {
		name   string
		update *map[string]string
		want   map[string]string
	}{
		{name: "omitted", want: map[string]string{"repository": "original/*"}},
		{name: "replace", update: &replacement, want: map[string]string{"ref": "refs/heads/release/*"}},
		{name: "empty object", update: &empty, want: map[string]string{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			existing := fnapi.StoredTrustRelationship{ClaimMatches: map[string]string{"repository": "original/*"}}
			assert.DeepEqual(t, updatedTrustRelationship(existing, trustRelationshipUpdate{claimMatches: tc.update}).ClaimMatches, tc.want)
		})
	}
}
