// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

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
		name         string
		include      []string
		includeGlobs []string
		excludeGlobs []string
		expected     []simpleEntry
	}{
		{"get all", nil, nil, nil, []simpleEntry{
			{"foo", true},
			{"foo/bar", false},
			{"foo/x", true},
			{"foo/x/y", false},
			{"foo/x/z.cue", false},
			{"quux", true},
			{"quux/baz.cue", false},
		}},
		{"include single file", []string{"foo/bar"}, nil, nil, []simpleEntry{
			{"foo", true},
			{"foo/bar", false},
		}},
		{"include glob", nil, []string{"*.cue"}, nil, []simpleEntry{
			{Name: "foo", IsDir: true},
			{Name: "foo/x", IsDir: true},
			{Name: "foo/x/z.cue", IsDir: false},
			{Name: "quux", IsDir: true},
			{Name: "quux/baz.cue", IsDir: false}},
		},
		{"exclude directory", nil, nil, []string{"x"}, []simpleEntry{
			{"foo", true},
			{"foo/bar", false},
			{"quux", true},
			{"quux/baz.cue", false},
		}},
	} {
		s, err := Snapshot(&inmem, SnapshotOpts{IncludeFiles: test.include, IncludeFilesGlobs: test.includeGlobs, ExcludeFilesGlobs: test.excludeGlobs})
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