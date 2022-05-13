// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

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
	return filepath.Join(dir, "fn"), nil
}

func Config() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return dir, err
	}
	return filepath.Join(dir, "fn"), nil
}

func SDKCache(name string) (string, error) {
	cacheDir, err := Cache()
	if err != nil {
		return "", err
	}

	return filepath.Join(cacheDir, "sdk", name), nil
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
