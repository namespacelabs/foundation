// Copyright 2026 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package metadata

import (
	"os"
	"path/filepath"
)

func Directory() (string, error) {
	if specified := os.Getenv("NSC_METADATA_DIR"); specified != "" {
		return specified, nil
	}
	return defaultDirectory()
}

func MetadataFile() (string, error) {
	return fileInDirectory("metadata.json")
}

func TokenFile() (string, error) {
	return fileInDirectory("token.json")
}

func TokenSpecFile() (string, error) {
	return fileInDirectory("token.spec")
}

func fileInDirectory(name string) (string, error) {
	dir, err := Directory()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name), nil
}
