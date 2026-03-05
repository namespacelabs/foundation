// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"namespacelabs.dev/foundation/internal/fnerrors"
)

func TestZipFiles(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "repro")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(wd)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	// Create structure
	// path/to/src/@generated/client/index.js
	// path/to/src/@generated/client/runtime/index.js
	// foo/@generated/bar.txt
	files := []string{
		"path/to/src/@generated/client/index.js",
		"path/to/src/@generated/client/runtime/index.js",
		"foo/@generated/bar.txt",
	}

	for _, f := range files {
		if err := os.MkdirAll(filepath.Dir(f), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(f, []byte("content"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Test cases
	testCases := []struct {
		pattern       string
		expectedCount int
	}{
		{"**/@generated/**", 3},
		{"**/@generated/**/*", 3},
		{"**/*.js", 2},
		{"**/client/**", 2}, // Matches client dir, walks it
	}

	for _, tc := range testCases {
		count, err := zipFiles(context.Background(), tc.pattern, io.Discard)
		if err != nil {
			t.Errorf("Pattern %q failed: %v", tc.pattern, err)
			continue
		}
		if count != tc.expectedCount {
			t.Errorf("Pattern %q: expected %d files, got %d", tc.pattern, tc.expectedCount, count)
		}
	}
}

func TestZipFilesWithSymlinks(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "symlink-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	srcDir := filepath.Join(tmpDir, "src")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatal(err)
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(wd)

	if err := os.Chdir(srcDir); err != nil {
		t.Fatal(err)
	}

	// Create a regular file
	if err := os.WriteFile("target.txt", []byte("hello world"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a subdirectory with another file
	if err := os.MkdirAll("subdir", 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("subdir/nested.txt", []byte("nested content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a symlink to the file
	if err := os.Symlink("target.txt", "link.txt"); err != nil {
		t.Fatal(err)
	}

	// Create a symlink to the directory
	if err := os.Symlink("subdir", "link_dir"); err != nil {
		t.Fatal(err)
	}

	// Zip all files
	var buf bytes.Buffer
	count, err := zipFiles(context.Background(), "**/*", &buf)
	if err != nil {
		t.Fatalf("zipFiles failed: %v", err)
	}

	// We expect: target.txt, subdir/nested.txt, link.txt (resolved), link_dir/nested.txt (resolved)
	// Note: symlinks are followed by os.Stat, so we get the content of the target
	if count < 2 {
		t.Errorf("Expected at least 2 files, got %d", count)
	}

	// Write zip to a temp file for unzipping
	zipPath := filepath.Join(tmpDir, "test.zip")
	if err := os.WriteFile(zipPath, buf.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}

	// Unzip to a new directory
	destDir := filepath.Join(tmpDir, "dest")
	if err := unzipArtifact(context.Background(), zipPath, destDir); err != nil {
		t.Fatalf("unzipArtifact failed: %v", err)
	}

	// Verify the target file exists
	content, err := os.ReadFile(filepath.Join(destDir, "target.txt"))
	if err != nil {
		t.Fatalf("Failed to read target.txt: %v", err)
	}
	if string(content) != "hello world" {
		t.Errorf("Expected 'hello world', got %q", string(content))
	}

	// Verify the nested file exists
	content, err = os.ReadFile(filepath.Join(destDir, "subdir", "nested.txt"))
	if err != nil {
		t.Fatalf("Failed to read subdir/nested.txt: %v", err)
	}
	if string(content) != "nested content" {
		t.Errorf("Expected 'nested content', got %q", string(content))
	}

	// Verify the symlink is recreated as a symlink
	linkPath := filepath.Join(destDir, "link.txt")
	linkInfo, err := os.Lstat(linkPath)
	if err != nil {
		t.Fatalf("Failed to lstat link.txt: %v", err)
	}
	if linkInfo.Mode()&os.ModeSymlink == 0 {
		t.Errorf("Expected link.txt to be a symlink, got mode %v", linkInfo.Mode())
	}

	// Verify the symlink target is correct
	target, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatalf("Failed to readlink link.txt: %v", err)
	}
	if target != "target.txt" {
		t.Errorf("Expected symlink target 'target.txt', got %q", target)
	}

	// Verify the symlink content is readable through the link
	content, err = os.ReadFile(linkPath)
	if err != nil {
		t.Fatalf("Failed to read link.txt: %v", err)
	}
	if string(content) != "hello world" {
		t.Errorf("Expected 'hello world' from link.txt, got %q", string(content))
	}

	// Verify directory symlink is recreated
	linkDirPath := filepath.Join(destDir, "link_dir")
	linkDirInfo, err := os.Lstat(linkDirPath)
	if err != nil {
		t.Fatalf("Failed to lstat link_dir: %v", err)
	}
	if linkDirInfo.Mode()&os.ModeSymlink == 0 {
		t.Errorf("Expected link_dir to be a symlink, got mode %v", linkDirInfo.Mode())
	}

	// Verify directory symlink target
	dirTarget, err := os.Readlink(linkDirPath)
	if err != nil {
		t.Fatalf("Failed to readlink link_dir: %v", err)
	}
	if dirTarget != "subdir" {
		t.Errorf("Expected symlink target 'subdir', got %q", dirTarget)
	}
}

func TestWriteArtifactDescribeNotFoundJSON(t *testing.T) {
	var buf bytes.Buffer

	msg, err := writeArtifactDescribeNotFound(&buf, "json", "foo/bar", "main")
	if err != nil {
		t.Fatalf("writeArtifactDescribeNotFound returned error: %v", err)
	}

	if msg != "foo/bar doesn't exist" {
		t.Fatalf("unexpected message: %q", msg)
	}

	var parsed artifactDescribeErrorJSONOutput
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if parsed.Error != msg {
		t.Errorf("unexpected error in JSON output: got %q, want %q", parsed.Error, msg)
	}
	if parsed.Path != "foo/bar" {
		t.Errorf("unexpected path in JSON output: got %q", parsed.Path)
	}
	if parsed.Namespace != "main" {
		t.Errorf("unexpected namespace in JSON output: got %q", parsed.Namespace)
	}
}

func TestWriteArtifactDescribeNotFoundPlain(t *testing.T) {
	var buf bytes.Buffer

	msg, err := writeArtifactDescribeNotFound(&buf, "plain", "foo/bar", "main")
	if err != nil {
		t.Fatalf("writeArtifactDescribeNotFound returned error: %v", err)
	}

	if msg != "foo/bar doesn't exist" {
		t.Fatalf("unexpected message: %q", msg)
	}

	if got, want := buf.String(), "foo/bar doesn't exist\n"; got != want {
		t.Fatalf("unexpected plain output: got %q, want %q", got, want)
	}
}

func TestWriteArtifactDescribeNotFoundInvalidOutput(t *testing.T) {
	var buf bytes.Buffer

	_, err := writeArtifactDescribeNotFound(&buf, "yaml", "foo/bar", "main")
	if err == nil {
		t.Fatalf("expected error for invalid output format")
	}

	if !fnerrors.IsOfKind(err, fnerrors.Kind_BADINPUT) {
		t.Fatalf("expected bad input error, got: %T (%v)", err, err)
	}
}

func TestArtifactDescribeNotFoundErrorRetainsExitCode(t *testing.T) {
	for _, output := range []string{"plain", "json"} {
		t.Run(output, func(t *testing.T) {
			var buf bytes.Buffer

			err := artifactDescribeNotFoundError(&buf, output, "foo/bar", "main")
			if err == nil {
				t.Fatalf("expected not-found error")
			}

			exitErr, ok := err.(fnerrors.ExitError)
			if !ok {
				t.Fatalf("expected fnerrors.ExitError, got %T (%v)", err, err)
			}

			if code := exitErr.ExitCode(); code != 2 {
				t.Fatalf("expected exit code 2, got %d", code)
			}
		})
	}
}
