// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package dirs

import (
	"os"
	"path/filepath"
)

func CreateUserTemp(dir, pattern string) (*os.File, error) {
	cacheDir, err := Cache()
	if err != nil {
		return nil, err
	}

	tmpDir := filepath.Join(cacheDir, "tmp", dir)

	// Make sure that the temp directory has permissions locked to the
	// owning user.
	if err := os.MkdirAll(tmpDir, 0700); err != nil {
		return nil, err
	}

	return os.CreateTemp(tmpDir, pattern)
}

func CreateUserTempDir(dir, pattern string) (string, error) {
	cacheDir, err := Cache()
	if err != nil {
		return "", err
	}

	tmpDir := filepath.Join(cacheDir, "tmp", dir)

	// Make sure that the temp directory has permissions locked to the
	// owning user.
	if err := os.MkdirAll(tmpDir, 0700); err != nil {
		return "", err
	}

	return os.MkdirTemp(tmpDir, pattern)
}
