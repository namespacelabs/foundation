// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package bazel

import (
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"gotest.tools/assert"
)

func TestSameGoRevision(t *testing.T) {
	const version = "v0.0.571-0.20260916092833-78be3afb5f5e"
	for _, tc := range []struct {
		goVersion string
		version   string
		want      bool
	}{
		{version, "78be3afb5f5e48ffad073b84cebac984a25835c9", true},
		{version, "78be3afb5f5e", true},
		{version, version, true},
		{version, "88be3afb5f5e48ffad073b84cebac984a25835c9", false},
		{version, "78be3afb", false},
		{version, "78be3afb5f5e-not-a-revision", false},
		{"v1.2.3", "v1.2.3", true},
		{"v1.2.3", "v1.2.4", false},
		{"", "", false},
	} {
		t.Run(tc.goVersion+"/"+tc.version, func(t *testing.T) {
			assert.Equal(t, tc.want, sameGoRevision(tc.goVersion, tc.version))
		})
	}
}

func TestGoRepositoryMetadata(t *testing.T) {
	const metadata = `{"canonicalName":"gazelle++go_deps+renamed_repo","apparentName":"@custom_alias","repoRuleName":"go_repository","attribute":[{"name":"importpath","stringValue":"example.com/acme"},{"name":"version","stringValue":"v1.2.3"}]}
{"canonicalName":"other+","repoRuleName":"http_archive","attribute":[{"name":"importpath","stringValue":"example.com/acme"},{"name":"version","stringValue":"v1.2.3"}]}`
	repos, err := parseGoRepositories(strings.NewReader(metadata))
	assert.NilError(t, err)
	assert.DeepEqual(t, repos, []goRepository{{name: "gazelle++go_deps+renamed_repo", module: "example.com/acme", version: "v1.2.3"}}, cmp.AllowUnexported(goRepository{}))
	name, err := matchingGoRepository(repos, "example.com/acme", "v1.2.3")
	assert.NilError(t, err)
	assert.Equal(t, "gazelle++go_deps+renamed_repo", name)
	name, err = matchingGoRepository(repos, "example.com/acme", "v1.2.4")
	assert.NilError(t, err)
	assert.Equal(t, name, "")
	name, err = matchingGoRepository(repos, "example.com/other", "v1.2.3")
	assert.NilError(t, err)
	assert.Equal(t, name, "")

	repos = append(repos, goRepository{name: "another_repo", module: "example.com/acme", version: "v1.2.3"})
	_, err = matchingGoRepository(repos, "example.com/acme", "v1.2.3")
	assert.ErrorContains(t, err, "multiple repositories")
	_, err = parseGoRepositories(strings.NewReader(metadata + "\n{"))
	assert.ErrorContains(t, err, "invalid repository metadata")
}

func TestGoRepositorySourceOverrides(t *testing.T) {
	for _, attr := range []string{
		`{"name":"replace","stringValue":"example.com/fork"}`,
		`{"name":"local_path","stringValue":"/work/fork"}`,
		`{"name":"remote","stringValue":"https://example.com/fork"}`,
		`{"name":"urls","stringListValue":["https://example.com/archive.zip"]}`,
	} {
		t.Run(attr, func(t *testing.T) {
			repos, err := parseGoRepositories(strings.NewReader(fmt.Sprintf(
				`{"canonicalName":"custom","repoRuleName":"go_repository","attribute":[{"name":"importpath","stringValue":"example.com/acme"},{"name":"version","stringValue":"v1.2.3"},%s]}`, attr)))
			assert.NilError(t, err)
			assert.Equal(t, len(repos), 0)
		})
	}
}
