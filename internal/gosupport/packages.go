// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package gosupport

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"golang.org/x/mod/modfile"
	"namespacelabs.dev/foundation/internal/fnerrors"
)

func ComputeGoPackage(parentPath string) (string, error) {
	f, gomodFile, err := LookupGoModule(parentPath)
	if err != nil {
		return "", err
	}

	rel, err := filepath.Rel(filepath.Dir(gomodFile), parentPath)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s/%s", f.Module.Mod.Path, rel), nil
}

func LookupGoModule(srcPath string) (*modfile.File, string, error) {
	gomodFile, err := findGoModule(srcPath)
	if err != nil {
		return nil, "", err
	}

	gomodBytes, err := ioutil.ReadFile(gomodFile)
	if err != nil {
		return nil, gomodFile, err
	}

	f, err := modfile.Parse(gomodFile, gomodBytes, nil)
	if err != nil {
		return nil, gomodFile, err
	}

	if f.Module == nil {
		return nil, gomodFile, fnerrors.UserError(nil, "%s: missing go module definition", gomodFile)
	}

	return f, gomodFile, nil
}

func findRoot(what, dir, presenceTest string, isDir bool) (string, error) {
	dir = filepath.Clean(dir)

	for {
		if fi, err := os.Stat(filepath.Join(dir, presenceTest)); err == nil && fi.IsDir() == isDir {
			return dir, nil
		}

		d := filepath.Dir(dir)
		if d == dir {
			return "", fnerrors.UserError(nil, "could not determine %s root", what)
		}
		dir = d
	}
}

func findGoModule(dir string) (string, error) {
	dir, err := findRoot("go module", dir, "go.mod", false)
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, "go.mod"), nil
}