// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package bazel

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/url"
	"os"
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
	mu              sync.Mutex
	repositories    map[string][]goRepository
	externalTargets map[string]string
	standalone      map[Target]*targetNode
}

func NewBuilder() *Builder {
	return &Builder{
		repositories:    map[string][]goRepository{},
		externalTargets: map[string]string{},
		standalone:      map[Target]*targetNode{},
	}
}

// Target describes a Bazel target that produces exactly one file or directory.
type Target struct {
	WorkspaceAbs string
	Label        string
	Platform     string
	OutputGroup  string
}

// AddTarget adds a Bazel target to the current build graph. When there is no
// graph, the target is finalized as a standalone invocation.
func (b *Builder) AddTarget(ctx context.Context, target Target) (compute.Computable[string], error) {
	graph, ok := build.GraphFromContext(ctx)
	if !ok {
		// Provisioning tools can be requested repeatedly before a build graph
		// exists. Share the computable so they do not start duplicate invocations.
		b.mu.Lock()
		defer b.mu.Unlock()
		if node := b.standalone[target]; node != nil {
			return node, nil
		}
		pass := &bazelGraph{}
		node, err := pass.add(target)
		if err != nil {
			return nil, err
		}
		if err := pass.Finalize(); err != nil {
			return nil, err
		}
		b.standalone[target] = node
		return node, nil
	}
	pass, err := graph.Pass("bazel", func() build.GraphPass {
		return &bazelGraph{}
	})
	if err != nil {
		return nil, err
	}
	return pass.(*bazelGraph).add(target)
}

type bazelGraph struct {
	mu sync.Mutex

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
	node := &targetNode{target: target}
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
	platform     string
	outputGroup  string
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
			platform:     node.target.Platform,
			outputGroup:  node.target.OutputGroup,
		}
		inv := invocations[key]
		if inv == nil {
			inv = &invocation{
				workspaceAbs: node.target.WorkspaceAbs,
				platform:     node.target.Platform,
				outputGroup:  node.target.OutputGroup,
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
	platform     string
	outputGroup  string
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
	return tasks.Action("bazel.build").Arg("targets", strings.Join(inv.targetList(), " ")).Arg("platform", inv.platform)
}

func (inv *invocation) Inputs() *compute.In {
	return compute.Inputs().
		Indigestible("workspace", inv.workspaceAbs).
		Str("platform", inv.platform).
		Str("outputGroup", inv.outputGroup).
		JSON("targets", inv.targetList())
}

func (inv *invocation) Output() compute.Output {
	return compute.Output{NonDeterministic: true, Unshareable: true}
}

func (inv *invocation) buildArgs() []string {
	var args []string
	if inv.platform != "" {
		// Bazel otherwise reuses output paths across --platforms invocations.
		// Concurrent platform builds must not overwrite each other's artifacts.
		args = append(args, "--platforms="+inv.platform, "--experimental_platform_in_output_dir")
	}
	if inv.outputGroup != "" {
		args = append(args, "--output_groups="+inv.outputGroup)
	}
	return args
}

func (inv *invocation) Compute(ctx context.Context, _ compute.Resolved) (outputs, error) {
	installation, err := bazelInstallation()
	if err != nil {
		return nil, err
	}
	targets := inv.targetList()
	canonical := make(map[string]string, len(targets))
	var repositoryMapping map[string]string
	for _, target := range targets {
		label := target
		if strings.HasPrefix(label, "@") && !strings.HasPrefix(label, "@@") {
			// Apparent names can be arbitrary aliases; use Bazel's repository
			// mapping rather than guessing the canonical name from a prefix.
			if repositoryMapping == nil {
				var stdout bytes.Buffer
				if err := runBazelWithOutput(ctx, installation, inv.workspaceAbs, &stdout, "mod", "dump_repo_mapping", ""); err != nil {
					return nil, err
				}
				if err := json.Unmarshal(stdout.Bytes(), &repositoryMapping); err != nil {
					return nil, fnerrors.Newf("bazel: failed to read repository mapping: %w", err)
				}
			}
			repo, suffix, ok := strings.Cut(label[1:], "//")
			mapped, found := repositoryMapping[repo]
			if !ok || !found {
				return nil, fnerrors.Newf("bazel: cannot resolve repository for %s", target)
			}
			label = "@@" + mapped + "//" + suffix
		}
		label = strings.TrimPrefix(label, "@@//")
		if !strings.Contains(label, "//") {
			label = "//" + label
		}
		if !strings.Contains(label, ":") {
			label += ":" + filepath.Base(label)
		}
		canonical[target] = label
	}
	events, err := os.CreateTemp("", "ns-bazel-build-events-*.json")
	if err != nil {
		return nil, fnerrors.Newf("bazel: failed to create build event file: %w", err)
	}
	eventsPath := events.Name()
	if err := events.Close(); err != nil {
		os.Remove(eventsPath)
		return nil, fnerrors.Newf("bazel: failed to close build event file: %w", err)
	}
	defer os.Remove(eventsPath)

	buildArgs := []string{"build", "--remote_download_outputs=toplevel", "--build_event_json_file=" + eventsPath, "--nobuild_event_json_file_path_conversion"}
	buildArgs = append(buildArgs, inv.buildArgs()...)
	buildArgs = append(buildArgs, targets...)
	if err := runBazel(ctx, installation, inv.workspaceAbs, buildArgs...); err != nil {
		return nil, err
	}
	f, err := os.Open(eventsPath)
	if err != nil {
		return nil, fnerrors.Newf("bazel: failed to open build event file: %w", err)
	}
	defer f.Close()
	return parseBuildEvents(inv.workspaceAbs, inv.outputGroup, canonical, f)
}

type buildEvent struct {
	ID struct {
		NamedSet struct {
			ID string `json:"id"`
		} `json:"namedSet"`
		TargetCompleted struct {
			Label string `json:"label"`
		} `json:"targetCompleted"`
	} `json:"id"`
	NamedSetOfFiles struct {
		Files    []buildEventFile `json:"files"`
		FileSets []struct {
			ID string `json:"id"`
		} `json:"fileSets"`
	} `json:"namedSetOfFiles"`
	Completed struct {
		DirectoryOutput []buildEventFile `json:"directoryOutput"`
		OutputGroup     []struct {
			Name     string `json:"name"`
			FileSets []struct {
				ID string `json:"id"`
			} `json:"fileSets"`
		} `json:"outputGroup"`
	} `json:"completed"`
}

type buildEventFile struct {
	Name       string   `json:"name"`
	URI        string   `json:"uri"`
	PathPrefix []string `json:"pathPrefix"`
}

type buildEventNamedSet struct {
	files    []buildEventFile
	children []string
}

func parseBuildEvents(workspaceAbs, outputGroup string, targets map[string]string, r io.Reader) (outputs, error) {
	if outputGroup == "" {
		outputGroup = "default"
	}
	namedSets := map[string]buildEventNamedSet{}
	targetSets := map[string][]string{}
	directories := map[string][]buildEventFile{}
	decoder := json.NewDecoder(r)
	for {
		var event buildEvent
		err := decoder.Decode(&event)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fnerrors.Newf("bazel: failed to read build events: %w", err)
		}
		if event.ID.NamedSet.ID != "" {
			set := buildEventNamedSet{files: event.NamedSetOfFiles.Files}
			for _, child := range event.NamedSetOfFiles.FileSets {
				set.children = append(set.children, child.ID)
			}
			namedSets[event.ID.NamedSet.ID] = set
		}
		if event.ID.TargetCompleted.Label != "" {
			directories[event.ID.TargetCompleted.Label] = event.Completed.DirectoryOutput
			for _, group := range event.Completed.OutputGroup {
				if group.Name == outputGroup {
					targetSets[event.ID.TargetCompleted.Label] = nil
					for _, set := range group.FileSets {
						targetSets[event.ID.TargetCompleted.Label] = append(targetSets[event.ID.TargetCompleted.Label], set.ID)
					}
				}
			}
		}
	}

	result := outputs{}
	for target, label := range targets {
		setIDs, ok := targetSets[label]
		if !ok {
			return nil, fnerrors.Newf("bazel: output for target %s is missing", target)
		}
		var files []buildEventFile
		seen := map[string]bool{}
		var collect func(string)
		collect = func(id string) {
			if seen[id] {
				return
			}
			seen[id] = true
			set := namedSets[id]
			files = append(files, set.files...)
			for _, child := range set.children {
				collect(child)
			}
		}
		for _, id := range setIDs {
			collect(id)
		}
		artifacts := map[string]struct{}{}
		// Output groups expand trees into children. directoryOutput retains
		// their declared roots, including empty trees and cross-package aliases.
		for _, directory := range directories[label] {
			path, err := buildEventFilePath(workspaceAbs, directory)
			if err != nil {
				return nil, err
			}
			// Directory entries are exec-root-relative, whereas their children
			// have absolute file URIs. Resolve the workspace's bazel-out link.
			path, err = filepath.EvalSymlinks(path)
			if err != nil {
				return nil, fnerrors.Newf("bazel: failed to resolve output directory: %w", err)
			}
			artifacts[path] = struct{}{}
		}
		for _, file := range files {
			path, err := buildEventFilePath(workspaceAbs, file)
			if err != nil {
				return nil, err
			}
			for root := range artifacts {
				if strings.HasPrefix(path, root+string(filepath.Separator)) {
					path = root
					break
				}
			}
			artifacts[path] = struct{}{}
		}
		if len(artifacts) != 1 {
			return nil, fnerrors.Newf("bazel: target %s produced %d files, expected one", target, len(artifacts))
		}
		for path := range artifacts {
			result[target] = path
		}
	}
	return result, nil
}

func buildEventFilePath(workspaceAbs string, file buildEventFile) (string, error) {
	if file.URI != "" {
		u, err := url.Parse(file.URI)
		if err != nil || u.Scheme != "file" {
			return "", fnerrors.Newf("bazel: unsupported output URI %q", file.URI)
		}
		return filepath.FromSlash(u.Path), nil
	}
	path := filepath.Join(append(file.PathPrefix, file.Name)...)
	if !filepath.IsAbs(path) {
		path = filepath.Join(workspaceAbs, path)
	}
	return path, nil
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
