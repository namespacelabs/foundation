// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package metadata

import (
	"path/filepath"
	"testing"
)

func TestFilesUseMetadataDirectoryOverride(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("NSC_METADATA_DIR", dir)

	metadataFile, err := MetadataFile()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, "metadata.json"); metadataFile != want {
		t.Fatalf("MetadataFile() = %q, want %q", metadataFile, want)
	}

	tokenFile, err := TokenFile()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, "token.json"); tokenFile != want {
		t.Fatalf("TokenFile() = %q, want %q", tokenFile, want)
	}

	tokenSpecFile, err := TokenSpecFile()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, "token.spec"); tokenSpecFile != want {
		t.Fatalf("TokenSpecFile() = %q, want %q", tokenSpecFile, want)
	}
}
