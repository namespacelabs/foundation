// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package bazel

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	"gotest.tools/assert"
	"namespacelabs.dev/foundation/internal/build"
)

func TestGraphCollapsesCompatibleTargets(t *testing.T) {
	graph := build.NewGraph()
	ctx := build.WithGraph(context.Background(), graph)
	builder := NewBuilder()

	first, err := builder.AddTarget(ctx, Target{WorkspaceAbs: "/workspace", Label: "//global/server/iam:iam", Platform: "linux_amd64"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := builder.AddTarget(ctx, Target{WorkspaceAbs: "/workspace", Label: "//registry/server:server", Platform: "linux_amd64"})
	if err != nil {
		t.Fatal(err)
	}
	differentPlatform, err := builder.AddTarget(ctx, Target{WorkspaceAbs: "/workspace", Label: "//global/server/iam:iam", Platform: "linux_arm64"})
	if err != nil {
		t.Fatal(err)
	}
	const externalLabel = "@@gazelle++go_deps+foundation//library/oss/postgres/prepare/clusterinstance:clusterinstance"
	var externalNodes []*targetNode
	for range 2 {
		external, err := builder.AddTarget(ctx, Target{WorkspaceAbs: "/workspace", Label: externalLabel, Platform: "linux_amd64"})
		assert.NilError(t, err)
		externalNodes = append(externalNodes, external.(*targetNode))
	}
	if err := graph.Finalize(); err != nil {
		t.Fatal(err)
	}

	firstNode := first.(*targetNode)
	secondNode := second.(*targetNode)
	differentPlatformNode := differentPlatform.(*targetNode)
	assert.Equal(t, firstNode.invocation.platform, "linux_amd64")
	assert.Equal(t, differentPlatformNode.invocation.platform, "linux_arm64")
	for _, node := range externalNodes {
		assert.Equal(t, node.invocation, firstNode.invocation)
	}
	if firstNode.invocation != secondNode.invocation {
		t.Fatal("compatible targets were assigned to different Bazel invocations")
	}
	if firstNode.invocation == differentPlatformNode.invocation {
		t.Fatal("targets with different arguments were assigned to the same Bazel invocation")
	}
	want := []string{"//global/server/iam:iam", "//registry/server:server", externalLabel}
	if got := firstNode.invocation.targetList(); !reflect.DeepEqual(got, want) {
		t.Fatalf("targets = %q, want %q", got, want)
	}
}

func TestTargetWithoutBuildGraphUsesStandaloneInvocation(t *testing.T) {
	got, err := NewBuilder().AddTarget(context.Background(), Target{WorkspaceAbs: "/workspace", Label: "//target"})
	if err != nil {
		t.Fatal(err)
	}
	if got.(*targetNode).invocation == nil {
		t.Fatal("target was not assigned to a standalone Bazel invocation")
	}
}

func TestStandaloneTargetsDeduplicateByWorkspaceLabelAndPlatform(t *testing.T) {
	builder := NewBuilder()
	ctx := context.Background()
	target := Target{WorkspaceAbs: "/workspace", Label: "@@foundation//prepare:prepare", Platform: "linux_amd64"}
	first, err := builder.AddTarget(ctx, target)
	assert.NilError(t, err)
	duplicate, err := builder.AddTarget(ctx, target)
	assert.NilError(t, err)
	assert.Equal(t, first, duplicate)
	for _, different := range []Target{
		{WorkspaceAbs: "/other", Label: target.Label, Platform: target.Platform},
		{WorkspaceAbs: target.WorkspaceAbs, Label: "@@foundation//other:other", Platform: target.Platform},
		{WorkspaceAbs: target.WorkspaceAbs, Label: target.Label, Platform: "linux_arm64"},
	} {
		node, err := builder.AddTarget(ctx, different)
		assert.NilError(t, err)
		assert.Assert(t, node != first)
	}
}

func TestInvocationBuildArgs(t *testing.T) {
	assert.DeepEqual(t, (&invocation{platform: "@rules_go//go/toolchain:linux_arm64"}).buildArgs(), []string{"--platforms=@rules_go//go/toolchain:linux_arm64"})
	assert.DeepEqual(t, (&invocation{}).buildArgs(), []string(nil))
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
