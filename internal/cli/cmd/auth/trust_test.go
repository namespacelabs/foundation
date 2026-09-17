// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package auth

import (
	"testing"

	"github.com/spf13/cobra"
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
