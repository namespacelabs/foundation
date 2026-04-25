// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBuildManifests(t *testing.T) {
	tempDir := t.TempDir()
	tag := "v1.2.3"
	version := "1.2.3"
	publishedAt := time.Unix(1714000000, 0).UTC()

	files := map[string]string{
		"ns_1.2.3_darwin_arm64.tar.gz":  "",
		"ns_1.2.3_linux_amd64.tar.gz":   "",
		"nsc_1.2.3_darwin_arm64.tar.gz": "",
		"nsc_1.2.3_linux_amd64.tar.gz":  "",
		"checksums.txt": "aaa ns_1.2.3_darwin_arm64.tar.gz\n" +
			"bbb ns_1.2.3_linux_amd64.tar.gz\n" +
			"ccc nsc_1.2.3_darwin_arm64.tar.gz\n" +
			"ddd nsc_1.2.3_linux_amd64.tar.gz\n",
	}

	for name, contents := range files {
		if err := os.WriteFile(filepath.Join(tempDir, name), []byte(contents), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	checksums, err := readChecksums(filepath.Join(tempDir, "checksums.txt"))
	if err != nil {
		t.Fatalf("readChecksums: %v", err)
	}

	manifests, tarballs, err := buildManifests(tempDir, version, tag, publishedAt, checksums)
	if err != nil {
		t.Fatalf("buildManifests: %v", err)
	}

	if len(tarballs) != 4 {
		t.Fatalf("got %d tarballs, want 4", len(tarballs))
	}

	if got := manifests["ns"].Version; got != tag {
		t.Fatalf("ns version = %q, want %q", got, tag)
	}

	if got := manifests["ns"].PublishedAt; !got.Equal(publishedAt) {
		t.Fatalf("published_at = %v, want %v", got, publishedAt)
	}

	if got := len(manifests["ns"].Artifacts); got != 2 {
		t.Fatalf("ns artifacts = %d, want 2", got)
	}

	if got := manifests["nsc"].Artifacts[1].SHA256; got != "ddd" {
		t.Fatalf("nsc linux checksum = %q, want %q", got, "ddd")
	}
}
