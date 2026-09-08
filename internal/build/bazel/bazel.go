// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package bazel

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/bazelbuild/bazelisk/core"
	"github.com/bazelbuild/bazelisk/repositories"
	"namespacelabs.dev/foundation/internal/build"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/std/tasks"
)

type Builder struct {
	bazelRC string
}

func NewBuilder(bazelRC string) *Builder {
	return &Builder{bazelRC: bazelRC}
}

// Target describes a Bazel target that produces exactly one output file.
type Target struct {
	WorkspaceAbs string
	Label        string
	BuildArgs    []string
}

// AddTarget adds a Bazel target to the current build graph. When there is no
// graph, the target is finalized as a standalone invocation.
func (b *Builder) AddTarget(ctx context.Context, target Target) (compute.Computable[string], error) {
	graph, ok := build.GraphFromContext(ctx)
	if !ok {
		pass := &bazelGraph{bazelRC: b.bazelRC}
		node, err := pass.add(target)
		if err != nil {
			return nil, err
		}
		if err := pass.Finalize(); err != nil {
			return nil, err
		}
		return node, nil
	}
	pass, err := graph.Pass("bazel:"+b.bazelRC, func() build.GraphPass {
		return &bazelGraph{bazelRC: b.bazelRC}
	})
	if err != nil {
		return nil, err
	}
	return pass.(*bazelGraph).add(target)
}

type bazelGraph struct {
	mu sync.Mutex

	bazelRC   string
	nodes     []*targetNode
	finalized bool
}

func (g *bazelGraph) add(target Target) (*targetNode, error) {
	if target.WorkspaceAbs == "" {
		return nil, fnerrors.InternalError("bazel: workspace is missing")
	}
	if target.Label == "" {
		return nil, fnerrors.InternalError("bazel: target label is missing")
	}
	node := &targetNode{target: Target{
		WorkspaceAbs: target.WorkspaceAbs,
		Label:        target.Label,
		BuildArgs:    append([]string(nil), target.BuildArgs...),
	}}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.finalized {
		return nil, fnerrors.InternalError("bazel: build graph has already been finalized")
	}
	g.nodes = append(g.nodes, node)
	return node, nil
}

type invocationKey struct {
	workspaceAbs string
	buildArgs    string
}

func (g *bazelGraph) Finalize() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.finalized {
		return fnerrors.InternalError("bazel: build graph has already been finalized")
	}
	g.finalized = true

	invocations := map[invocationKey]*invocation{}
	for _, node := range g.nodes {
		key := invocationKey{
			workspaceAbs: node.target.WorkspaceAbs,
			buildArgs:    strings.Join(node.target.BuildArgs, "\x00"),
		}
		inv := invocations[key]
		if inv == nil {
			inv = &invocation{
				workspaceAbs: node.target.WorkspaceAbs,
				bazelRC:      g.bazelRC,
				buildArgs:    append([]string(nil), node.target.BuildArgs...),
				targets:      map[string]struct{}{},
			}
			invocations[key] = inv
		}
		inv.targets[node.target.Label] = struct{}{}
		node.invocation = inv
	}
	return nil
}

type targetNode struct {
	target     Target
	invocation *invocation

	compute.LocalScoped[string]
}

func (n *targetNode) Action() *tasks.ActionEvent {
	return tasks.Action("bazel.collect-output").Arg("target", n.target.Label)
}

func (n *targetNode) Inputs() *compute.In {
	if n.invocation == nil {
		panic("Bazel build graph was not finalized")
	}
	return compute.Inputs().Str("target", n.target.Label).Computable("invocation", n.invocation)
}

func (n *targetNode) Output() compute.Output {
	return compute.Output{NonDeterministic: true, Unshareable: true}
}

func (n *targetNode) Compute(_ context.Context, deps compute.Resolved) (string, error) {
	outputs := compute.MustGetDepValue(deps, n.invocation, "invocation")
	output := outputs[n.target.Label]
	if output == "" {
		return "", fnerrors.InternalError("bazel: output for target %s is missing", n.target.Label)
	}
	return output, nil
}

type outputs map[string]string

type invocation struct {
	workspaceAbs string
	bazelRC      string
	buildArgs    []string
	targets      map[string]struct{}

	compute.LocalScoped[outputs]
}

func (inv *invocation) targetList() []string {
	targets := make([]string, 0, len(inv.targets))
	for target := range inv.targets {
		targets = append(targets, target)
	}
	sort.Strings(targets)
	return targets
}

func (inv *invocation) Action() *tasks.ActionEvent {
	return tasks.Action("bazel.build").Arg("targets", strings.Join(inv.targetList(), " ")).Arg("args", strings.Join(inv.buildArgs, " "))
}

func (inv *invocation) Inputs() *compute.In {
	return compute.Inputs().
		Indigestible("workspace", inv.workspaceAbs).
		Str("bazelrc", inv.bazelRC).
		JSON("args", inv.buildArgs).
		JSON("targets", inv.targetList())
}

func (inv *invocation) Output() compute.Output {
	return compute.Output{NonDeterministic: true, Unshareable: true}
}

func (inv *invocation) Compute(ctx context.Context, _ compute.Resolved) (outputs, error) {
	installation, err := bazelInstallation()
	if err != nil {
		return nil, err
	}
	targets := inv.targetList()
	startup := []string{"--bazelrc=" + inv.bazelRC}
	buildArgs := append([]string{}, startup...)
	buildArgs = append(buildArgs, "build", "--remote_download_outputs=all")
	buildArgs = append(buildArgs, inv.buildArgs...)
	buildArgs = append(buildArgs, targets...)
	if err := runBazel(ctx, installation, inv.workspaceAbs, buildArgs...); err != nil {
		return nil, err
	}

	result := outputs{}
	for _, target := range targets {
		var stdout bytes.Buffer
		cqueryArgs := append([]string{}, startup...)
		cqueryArgs = append(cqueryArgs, "cquery", "--output=files")
		cqueryArgs = append(cqueryArgs, inv.buildArgs...)
		cqueryArgs = append(cqueryArgs, target)
		if err := runBazelWithOutput(ctx, installation, inv.workspaceAbs, &stdout, cqueryArgs...); err != nil {
			return nil, err
		}
		output, err := parseOutput(inv.workspaceAbs, target, stdout.String())
		if err != nil {
			return nil, err
		}
		result[target] = output
	}
	return result, nil
}

func bazelInstallation() (string, error) {
	config := core.MakeDefaultConfig()
	gcs := &repositories.GCSRepo{}
	github := repositories.CreateGitHubRepo(config.Get("BAZELISK_GITHUB_TOKEN"))
	repos := core.CreateRepositories(gcs, github, gcs, gcs, true)
	installation, err := core.GetBazelInstallation(repos, config)
	if err != nil {
		return "", fnerrors.Newf("bazel: failed to install Bazel: %w", err)
	}
	return installation.Path, nil
}

func parseOutput(workspaceAbs, target, output string) (string, error) {
	var files []string
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		files = append(files, line)
	}
	if err := scanner.Err(); err != nil {
		return "", fnerrors.Newf("bazel: failed to read cquery output: %w", err)
	}
	if len(files) != 1 {
		return "", fnerrors.Newf("bazel: target %s produced %d files, expected one", target, len(files))
	}
	source := files[0]
	if !filepath.IsAbs(source) {
		source = filepath.Join(workspaceAbs, source)
	}
	return source, nil
}

func runBazel(ctx context.Context, binary, dir string, args ...string) error {
	return runBazelWithOutput(ctx, binary, dir, console.Output(ctx, "bazel"), args...)
}

func runBazelWithOutput(ctx context.Context, binary, dir string, stdout io.Writer, args ...string) error {
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = dir
	cmd.Stdout = stdout
	cmd.Stderr = console.Output(ctx, "bazel")
	if err := cmd.Run(); err != nil {
		return fnerrors.Newf("bazel: command failed: %w", err)
	}
	return nil
}
