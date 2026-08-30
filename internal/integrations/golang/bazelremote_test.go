// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package golang

import (
	"archive/tar"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWriteWorkspaceTar(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("main.go", filepath.Join(root, "src", "main-link.go")); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".git", "objects"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".git", "objects", "ignored"), []byte("ignored"), 0o644); err != nil {
		t.Fatal(err)
	}

	first := filepath.Join(t.TempDir(), "first.tar")
	second := filepath.Join(t.TempDir(), "second.tar")
	if err := writeWorkspaceTar(first, root); err != nil {
		t.Fatalf("writeWorkspaceTar: %v", err)
	}
	if err := writeWorkspaceTar(second, root); err != nil {
		t.Fatalf("writeWorkspaceTar: %v", err)
	}
	firstContents, err := os.ReadFile(first)
	if err != nil {
		t.Fatal(err)
	}
	secondContents, err := os.ReadFile(second)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstContents, secondContents) {
		t.Error("archive is not deterministic")
	}

	entries := map[string]*tar.Header{}
	r := tar.NewReader(bytes.NewReader(firstContents))
	for {
		header, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		entries[header.Name] = header
		if header.Name == "src/main.go" {
			contents, err := io.ReadAll(r)
			if err != nil {
				t.Fatal(err)
			}
			if got, want := string(contents), "package main\n"; got != want {
				t.Errorf("src/main.go contents = %q, want %q", got, want)
			}
		}
	}
	if _, ok := entries[".git"]; ok {
		t.Error("archive includes .git")
	}
	if got := entries["src/main-link.go"].Linkname; got != "main.go" {
		t.Errorf("symlink target = %q, want %q", got, "main.go")
	}
	for name, header := range entries {
		if header.Uid != 0 || header.Gid != 0 || header.Uname != "" || header.Gname != "" {
			t.Errorf("%s has non-normalized ownership: %+v", name, header)
		}
		if got, want := header.ModTime, time.Unix(1, 0); !got.Equal(want) {
			t.Errorf("%s modtime = %s, want %s", name, got, want)
		}
	}
}
