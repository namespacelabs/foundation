// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package bazel

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	"namespacelabs.dev/foundation/internal/build"
)

func TestGraphCollapsesCompatibleTargets(t *testing.T) {
	graph := build.NewGraph()
	ctx := build.WithGraph(context.Background(), graph)
	builder := NewBuilder("/tmp/namespace.bazelrc")

	first, err := builder.AddTarget(ctx, Target{WorkspaceAbs: "/workspace", Label: "//global/server/iam:iam", BuildArgs: []string{"--platforms=linux_amd64"}})
	if err != nil {
		t.Fatal(err)
	}
	second, err := builder.AddTarget(ctx, Target{WorkspaceAbs: "/workspace", Label: "//registry/server:server", BuildArgs: []string{"--platforms=linux_amd64"}})
	if err != nil {
		t.Fatal(err)
	}
	differentPlatform, err := builder.AddTarget(ctx, Target{WorkspaceAbs: "/workspace", Label: "//global/server/iam:iam", BuildArgs: []string{"--platforms=linux_arm64"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := graph.Finalize(); err != nil {
		t.Fatal(err)
	}

	firstNode := first.(*targetNode)
	secondNode := second.(*targetNode)
	differentPlatformNode := differentPlatform.(*targetNode)
	if firstNode.invocation != secondNode.invocation {
		t.Fatal("compatible targets were assigned to different Bazel invocations")
	}
	if firstNode.invocation == differentPlatformNode.invocation {
		t.Fatal("targets with different arguments were assigned to the same Bazel invocation")
	}
	want := []string{"//global/server/iam:iam", "//registry/server:server"}
	if got := firstNode.invocation.targetList(); !reflect.DeepEqual(got, want) {
		t.Fatalf("targets = %q, want %q", got, want)
	}
}

func TestTargetWithoutBuildGraphUsesStandaloneInvocation(t *testing.T) {
	got, err := NewBuilder("/tmp/namespace.bazelrc").AddTarget(context.Background(), Target{WorkspaceAbs: "/workspace", Label: "//target"})
	if err != nil {
		t.Fatal(err)
	}
	if got.(*targetNode).invocation == nil {
		t.Fatal("target was not assigned to a standalone Bazel invocation")
	}
}

func TestGraphRejectsTargetAfterFinalize(t *testing.T) {
	graph := &bazelGraph{}
	if err := graph.Finalize(); err != nil {
		t.Fatal(err)
	}
	if _, err := graph.add(Target{WorkspaceAbs: "/workspace", Label: "//target"}); err == nil {
		t.Fatal("expected adding a target after finalization to fail")
	}
}

func TestParseOutputPreservesRequestedTargetIdentity(t *testing.T) {
	got, err := parseOutput("/workspace", "//global/server/iam:alias", "bazel-bin/global/server/iam/iam\n")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join("/workspace", "bazel-bin/global/server/iam/iam")
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("outputs = %q, want %q", got, want)
	}
}

func TestParseOutputRejectsMultipleFiles(t *testing.T) {
	if _, err := parseOutput("/workspace", "//target", "bazel-bin/first\nbazel-bin/second\n"); err == nil {
		t.Fatal("expected multiple outputs to fail")
	}
}
