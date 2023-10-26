// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package dirs

import (
	"os"
	"path/filepath"
	"strings"
)

func Cache() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return dir, err
	}
	return filepath.Join(dir, "ns"), nil
}

func Config() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return dir, err
	}

	// Best-effort to migrate configuration to the "ns" directory.
	// TODO: Remove after M0 release.
	oldPath := filepath.Join(dir, "fn")
	newPath := filepath.Join(dir, "ns")
	_, oldErr := os.Stat(oldPath)
	_, newErr := os.Stat(newPath)
	if os.IsNotExist(newErr) && oldErr == nil {
		_ = os.Rename(oldPath, newPath)
	}

	return filepath.Join(dir, "ns"), nil
}

func ModuleCache(name, ref string) (string, error) {
	cacheDir, err := ModuleCacheRoot()
	if err != nil {
		return "", err
	}

	return filepath.Join(cacheDir, name, ref), nil
}

func ModuleCacheRoot() (string, error) {
	cacheDir, err := Cache()
	if err != nil {
		return "", err
	}

	return filepath.Join(cacheDir, "module"), nil
}

func CertCache() (string, error) {
	cacheDir, err := Cache()
	if err != nil {
		return "", err
	}

	return filepath.Join(cacheDir, "certs"), nil
}

func UnpackCache() (string, error) {
	cacheDir, err := Cache()
	if err != nil {
		return "", err
	}

	return filepath.Join(cacheDir, "unpack"), nil
}

func Subdir(rel string) (string, error) {
	cacheDir, err := Cache()
	if err != nil {
		return "", err
	}

	return filepath.Join(cacheDir, rel), nil
}

func Logs(rel string) (string, error) {
	cacheDir, err := Cache()
	if err != nil {
		return "", err
	}

	return filepath.Join(cacheDir, "log", rel), nil
}

func Ensure(dir string, err error) (string, error) {
	if err != nil {
		return dir, err
	}

	return dir, os.MkdirAll(dir, 0755)
}

func ExpandHome(path string) (string, error) {
	if !strings.HasPrefix(path, "~") {
		return path, nil
	}

	dir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, path[1:]), nil
}
