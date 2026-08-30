// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package orchestration

import (
	"context"
	"testing"

	"namespacelabs.dev/foundation/internal/build/binary"
	"namespacelabs.dev/foundation/orchestration/server/constants"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/pkggraph"
)

func TestOrchestratorToolPrebuiltFollowsMode(t *testing.T) {
	base := cfg.MakeConfigurationWith("test", cfg.MakeSyntheticWorkspace(&schema.Workspace{}, nil), cfg.ConfigurationSlice{})
	module := pkggraph.NewModule(&schema.Workspace{}, &schema.Workspace_LoadedFrom{}, "")
	tool := pkggraph.NewLocationForTesting(module, constants.ToolPkg.String(), "orchestration/server/tool")

	t.Run("prebuilt", func(t *testing.T) {
		env, err := MakeOrchestratorContext(context.Background(), base, OrchestratorModePrebuilt)
		if err != nil {
			t.Fatal(err)
		}

		got, err := binary.PrebuiltImageID(context.Background(), tool, env.Configuration())
		if err != nil {
			t.Fatal(err)
		}
		if got == nil {
			t.Fatal("expected the orchestrator tool to resolve to a prebuilt image")
		}
		want := PrebuiltOrchestratorToolRepository + "@" + PrebuiltOrchestratorToolDigest
		if got.RepoAndDigest() != want {
			t.Fatalf("unexpected orchestrator tool image: got %q, want %q", got.RepoAndDigest(), want)
		}
	})

	t.Run("head", func(t *testing.T) {
		env, err := MakeOrchestratorContext(context.Background(), base, OrchestratorModeHead)
		if err != nil {
			t.Fatal(err)
		}

		got, err := binary.PrebuiltImageID(context.Background(), tool, env.Configuration())
		if err != nil {
			t.Fatal(err)
		}
		if got != nil {
			t.Fatalf("head mode unexpectedly resolved a prebuilt: %s", got.RepoAndDigest())
		}
	})
}
