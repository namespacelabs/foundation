// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package bazel

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"golang.org/x/mod/module"
	"namespacelabs.dev/foundation/internal/fnerrors"
)

type goRepository struct {
	name    string
	module  string
	version string
}

// ExternalGoTarget resolves a Go target against the invoking workspace, not the
// downloaded Namespace module. Unmapped or differently versioned modules retain
// the direct-Go path rather than silently building another revision.
func (b *Builder) ExternalGoTarget(ctx context.Context, workspaceAbs, modulePath, version, target string) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	key := strings.Join([]string{workspaceAbs, modulePath, version, target}, "\x00")
	if label, ok := b.externalTargets[key]; ok {
		return label, nil
	}
	installation, err := bazelInstallation()
	if err != nil {
		return "", err
	}
	repos, ok := b.repositories[workspaceAbs]
	if !ok {
		var stdout bytes.Buffer
		if err := runBazelWithOutput(ctx, installation, workspaceAbs, &stdout,
			"mod", "show_repo", "--all_visible_repos", "--output=streamed_jsonproto"); err != nil {
			return "", err
		}
		repos, err = parseGoRepositories(&stdout)
		if err != nil {
			return "", err
		}
		b.repositories[workspaceAbs] = repos
	}

	repo, err := matchingGoRepository(repos, modulePath, version)
	if err != nil {
		return "", err
	}
	if repo == "" {
		b.externalTargets[key] = ""
		return "", nil
	}

	label := "@@" + repo + target
	pkg, _, _ := strings.Cut(label, ":")
	var stdout bytes.Buffer
	if err := runBazelWithOutput(ctx, installation, workspaceAbs, &stdout,
		"query", "--consistent_labels", "--output=label", "kind('go_binary rule', "+pkg+":*)"); err != nil {
		return "", err
	}
	for _, found := range strings.Fields(stdout.String()) {
		if found == label {
			b.externalTargets[key] = label
			return label, nil
		}
	}
	b.externalTargets[key] = ""
	return "", nil
}

func parseGoRepositories(r io.Reader) ([]goRepository, error) {
	decoder := json.NewDecoder(r)
	var repos []goRepository
	for {
		var repo struct {
			CanonicalName string `json:"canonicalName"`
			RepoRuleName  string `json:"repoRuleName"`
			Attribute     []struct {
				Name            string   `json:"name"`
				StringValue     string   `json:"stringValue"`
				StringListValue []string `json:"stringListValue"`
			} `json:"attribute"`
		}
		if err := decoder.Decode(&repo); errors.Is(err, io.EOF) {
			return repos, nil
		} else if err != nil {
			return nil, fnerrors.Newf("bazel: invalid repository metadata: %w", err)
		}
		if repo.RepoRuleName != "go_repository" {
			continue
		}
		attrs := map[string]string{}
		var sourceOverride bool
		for _, attr := range repo.Attribute {
			attrs[attr.Name] = attr.StringValue
			switch attr.Name {
			case "replace", "local_path", "remote", "urls":
				sourceOverride = sourceOverride || attr.StringValue != "" || len(attr.StringListValue) > 0
			}
		}
		// Overrides need an independent source-identity check before they can
		// substitute for the Namespace module's sources.
		if sourceOverride || repo.CanonicalName == "" || attrs["importpath"] == "" {
			continue
		}
		version := attrs["version"]
		if version == "" {
			version = attrs["commit"]
		}
		if version == "" {
			version = attrs["tag"]
		}
		repos = append(repos, goRepository{name: repo.CanonicalName, module: attrs["importpath"], version: version})
	}
}

func matchingGoRepository(repos []goRepository, modulePath, version string) (string, error) {
	var name string
	for _, repo := range repos {
		if repo.module != modulePath || !sameGoRevision(repo.version, version) {
			continue
		}
		if name != "" && name != repo.name {
			return "", fnerrors.Newf("bazel: multiple repositories match %s@%s", modulePath, version)
		}
		name = repo.name
	}
	return name, nil
}

func sameGoRevision(goVersion, version string) bool {
	if goVersion == "" || version == "" {
		return false
	}
	if goVersion == version {
		return true
	}
	revision, err := module.PseudoVersionRev(goVersion)
	if err != nil || len(revision) < 12 || len(version) < len(revision) {
		return false
	}
	if _, err := hex.DecodeString(version); err != nil {
		return false
	}
	return strings.HasPrefix(version, revision)
}
