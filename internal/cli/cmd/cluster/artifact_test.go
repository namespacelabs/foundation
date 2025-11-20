// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
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
