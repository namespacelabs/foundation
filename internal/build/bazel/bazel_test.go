// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package bazel

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"gotest.tools/assert"
	"namespacelabs.dev/foundation/internal/build"
	"namespacelabs.dev/foundation/internal/compute"
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
	image, err := builder.AddTarget(ctx, Target{WorkspaceAbs: "/workspace", Label: "//global/server/iam:iam", Platform: "linux_amd64", OutputGroup: "oci_tarball"})
	assert.NilError(t, err)
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
	assert.Assert(t, image.(*targetNode).invocation != firstNode.invocation)
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
	assert.DeepEqual(t, (&invocation{platform: "@rules_go//go/toolchain:linux_arm64"}).buildArgs(), []string{"--platforms=@rules_go//go/toolchain:linux_arm64", "--experimental_platform_in_output_dir"})
	assert.DeepEqual(t, (&invocation{platform: "//platforms:arm64", outputGroup: "oci_tarball"}).buildArgs(), []string{"--platforms=//platforms:arm64", "--experimental_platform_in_output_dir", "--output_groups=oci_tarball"})
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

func TestInvocationDiscoversBuildOutputs(t *testing.T) {
	workspace := t.TempDir()
	installation, err := bazelInstallation()
	assert.NilError(t, err)
	t.Cleanup(func() { assert.NilError(t, runBazel(context.Background(), installation, workspace, "shutdown")) })
	writeFixture := func(path, contents string) {
		t.Helper()
		path = filepath.Join(workspace, path)
		assert.NilError(t, os.MkdirAll(filepath.Dir(path), 0755))
		assert.NilError(t, os.WriteFile(path, []byte(contents), 0644))
	}
	writeFixture("MODULE.bazel", `module(name = "fixture")
bazel_dep(name = "output_fixture", version = "1.0", repo_name = "renamed")
local_path_override(module_name = "output_fixture", path = "external")
`)
	writeFixture("BUILD.bazel", `genrule(name = "main", outs = ["nested/main.txt"], cmd = "echo main > $@")
alias(name = "main_alias", actual = ":main")
genrule(name = "multiple", outs = ["one.txt", "two.txt"], cmd = "touch $(OUTS)")
`)
	writeFixture("external/MODULE.bazel", `module(name = "output_fixture", version = "1.0")
`)
	writeFixture("external/BUILD.bazel", `genrule(name = "external", outs = ["external.txt"], cmd = "echo external > $@")
alias(name = "external_alias", actual = ":external")
`)

	targets := []string{"//:main_alias", "@renamed//:external_alias", "@@output_fixture+//:external_alias"}
	got, err := (&invocation{workspaceAbs: workspace, targets: map[string]struct{}{
		targets[0]: {}, targets[1]: {}, targets[2]: {},
	}}).Compute(context.Background(), compute.Resolved{})
	assert.NilError(t, err)
	for _, target := range targets {
		assert.Assert(t, filepath.IsAbs(got[target]), target)
		contents, err := os.ReadFile(got[target])
		assert.NilError(t, err)
		if target == targets[0] {
			assert.Equal(t, string(contents), "main\n")
		} else {
			assert.Equal(t, string(contents), "external\n")
		}
	}
	_, err = (&invocation{workspaceAbs: workspace, targets: map[string]struct{}{"//:multiple": {}}}).Compute(context.Background(), compute.Resolved{})
	assert.ErrorContains(t, err, "produced 2 files, expected one")
}

func TestInvocationDiscoversExplicitOutputGroupDirectory(t *testing.T) {
	workspace := t.TempDir()
	installation, err := bazelInstallation()
	assert.NilError(t, err)
	t.Cleanup(func() { assert.NilError(t, runBazel(context.Background(), installation, workspace, "shutdown")) })
	assert.NilError(t, os.WriteFile(filepath.Join(workspace, "MODULE.bazel"), []byte(`module(name = "fixture")
`), 0644))
	assert.NilError(t, os.WriteFile(filepath.Join(workspace, "rule.bzl"), []byte(`def _directory(ctx):
    out = ctx.actions.declare_directory("nested/" + ctx.label.name + ".dir")
    command = "mkdir -p %s" % out.path
    if ctx.label.name != "empty":
        command += " && echo nested > %s/file && echo other > %s/other" % (out.path, out.path)
    ctx.actions.run_shell(outputs = [out], command = command)
    return [DefaultInfo(), OutputGroupInfo(oci_tarball = depset(transitive = [depset([out])]))]

directory = rule(implementation = _directory)
`), 0644))
	assert.NilError(t, os.WriteFile(filepath.Join(workspace, "BUILD.bazel"), []byte(`load(":rule.bzl", "directory")
directory(name = "image", visibility = ["//visibility:public"])
directory(name = "empty")
`), 0644))
	assert.NilError(t, os.MkdirAll(filepath.Join(workspace, "alias"), 0755))
	assert.NilError(t, os.WriteFile(filepath.Join(workspace, "alias/BUILD.bazel"), []byte(`alias(name = "image", actual = "//:image")
`), 0644))

	const target = "//alias:image"
	got, err := (&invocation{workspaceAbs: workspace, outputGroup: "oci_tarball", targets: map[string]struct{}{target: {}, "//:empty": {}}}).Compute(context.Background(), compute.Resolved{})
	assert.NilError(t, err)
	info, err := os.Stat(got[target])
	assert.NilError(t, err)
	assert.Assert(t, info.IsDir(), "output %q has mode %v", got[target], info.Mode())
	contents, err := os.ReadFile(filepath.Join(got[target], "file"))
	assert.NilError(t, err)
	assert.Equal(t, string(contents), "nested\n")
	entries, err := os.ReadDir(got["//:empty"])
	assert.NilError(t, err)
	assert.Equal(t, len(entries), 0)
}
