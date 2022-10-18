// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package memfs

import (
	"io/fs"
	"log"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"namespacelabs.dev/foundation/internal/fnfs"
)

func TestReadDir(t *testing.T) {
	var inmem FS

	inmem.Add("foo/bar", []byte("1"))
	inmem.Add("quux/baz", []byte("2"))
	inmem.Add("foo/x/y", []byte("3"))
	inmem.Add("foo/x/z", []byte("4"))

	one, _ := inmem.ReadDir(".")
	if d := cmp.Diff([]simpleEntry{
		{"foo", true},
		{"quux", true},
	}, names(one)); d != "" {
		t.Errorf("mismatch (-want +got):\n%s", d)
	}

	two, _ := inmem.ReadDir("foo")
	if d := cmp.Diff([]simpleEntry{
		{"bar", false},
		{"x", true},
	}, names(two)); d != "" {
		t.Errorf("mismatch (-want +got):\n%s", d)
	}

	three, _ := inmem.ReadDir("quux")
	if d := cmp.Diff([]simpleEntry{
		{"baz", false},
	}, names(three)); d != "" {
		t.Errorf("mismatch (-want +got):\n%s", d)
	}

	four, _ := inmem.ReadDir("foo/x")
	if d := cmp.Diff([]simpleEntry{
		{"y", false},
		{"z", false},
	}, names(four)); d != "" {
		t.Errorf("mismatch (-want +got):\n%s", d)
	}

	five, _ := inmem.ReadDir("doesntexist")
	if d := cmp.Diff([]simpleEntry{}, names(five)); d != "" {
		t.Errorf("mismatch (-want +got):\n%s", d)
	}

	sub, _ := fs.Sub(&inmem, "foo/x")
	six, _ := fs.ReadDir(sub, ".")
	if d := cmp.Diff([]simpleEntry{
		{"y", false},
		{"z", false},
	}, names(six)); d != "" {
		t.Errorf("mismatch (-want +got):\n%s", d)
	}
}

func TestRemove(t *testing.T) {
	var inmem FS

	inmem.Add("foo/y", []byte("1"))
	inmem.Add("foo/x/z", []byte("2"))
	inmem.Add("foo/x/w", []byte("3"))

	if err := inmem.Remove("foo/x/z"); err != nil {
		t.Fatal(err)
	}

	visited, err := visitAll(&inmem)
	if err != nil {
		t.Fatal(err)
	}

	if d := cmp.Diff([]simpleEntry{
		{Name: "foo", IsDir: true},
		{Name: "foo/x", IsDir: true},
		{Name: "foo/x/w"},
		{Name: "foo/y"},
	}, visited); d != "" {
		t.Errorf("mismatch (-want +got):\n%s", d)
	}
}

type simpleEntry struct {
	Name  string
	IsDir bool
}

func names(entries []os.DirEntry) []simpleEntry {
	names := []simpleEntry{}
	for _, ent := range entries {
		names = append(names, simpleEntry{ent.Name(), ent.IsDir()})
	}
	return names
}

func TestWalk(t *testing.T) {
	var inmem FS

	inmem.Add("foo/bar", []byte("1"))
	inmem.Add("quux/baz", []byte("2"))
	inmem.Add("foo/x/y", []byte("3"))
	inmem.Add("foo/x/z", []byte("4"))

	visited, err := visitAll(&inmem)
	if err != nil {
		t.Fatal(err)
	}

	if d := cmp.Diff([]simpleEntry{
		{"foo", true},
		{"foo/bar", false},
		{"foo/x", true},
		{"foo/x/y", false},
		{"foo/x/z", false},
		{"quux", true},
		{"quux/baz", false},
	}, visited); d != "" {
		t.Errorf("mismatch (-want +got):\n%s", d)
	}
}

func visitAll(fsys fs.ReadDirFS) ([]simpleEntry, error) {
	var visited []simpleEntry
	if err := fnfs.WalkDir(fsys, ".", func(path string, d fs.DirEntry) error {
		if path == "." {
			return nil
		}
		visited = append(visited, simpleEntry{path, d.IsDir()})
		return nil
	}); err != nil {
		return nil, err
	}
	return visited, nil
}

func TestStatDir(t *testing.T) {
	var inmem FS

	inmem.Add("foo/bar", []byte("1"))

	st, err := fs.Stat(&inmem, "foo")
	if err != nil {
		log.Fatal(err)
	}

	if !st.IsDir() {
		log.Fatal("expected directory to be a directory")
	}
}
