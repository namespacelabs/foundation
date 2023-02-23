// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package memfs

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestSnapshot(t *testing.T) {
	var inmem FS

	inmem.Add("foo/bar", []byte("1"))
	inmem.Add("quux/baz.cue", []byte("2"))
	inmem.Add("foo/x/y", []byte("3"))
	inmem.Add("foo/x/z.cue", []byte("4"))

	for _, test := range []struct {
		name            string
		include         []string
		excludePatterns []string
		expected        []simpleEntry
	}{
		{"get all", nil, nil, []simpleEntry{
			{"foo", true},
			{"foo/bar", false},
			{"foo/x", true},
			{"foo/x/y", false},
			{"foo/x/z.cue", false},
			{"quux", true},
			{"quux/baz.cue", false},
		}},
		{"include single file", []string{"foo/bar"}, nil, []simpleEntry{
			{"foo", true},
			{"foo/bar", false},
		}},
		{"include all CUE", nil, []string{"*", "!**/*.cue"}, []simpleEntry{
			{Name: "foo", IsDir: true},
			{Name: "foo/x", IsDir: true},
			{Name: "foo/x/z.cue", IsDir: false},
			{Name: "quux", IsDir: true},
			{Name: "quux/baz.cue", IsDir: false}},
		},
		{"exclude directory", nil, []string{"*/x"}, []simpleEntry{
			{"foo", true},
			{"foo/bar", false},
			{"quux", true},
			{"quux/baz.cue", false},
		}},
	} {
		s, err := Snapshot(&inmem, SnapshotOpts{IncludeFiles: test.include, ExcludePatterns: test.excludePatterns})
		if err != nil {
			t.Error(err)
			continue
		}

		visited, err := visitAll(s)
		if err != nil {
			t.Fatal(err)
		}

		if d := cmp.Diff(test.expected, visited); d != "" {
			t.Errorf("%s: mismatch (-want +got):\n%s", test.name, d)
		}
	}
}
