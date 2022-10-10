// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package gosupport

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/mod/modfile"
	"namespacelabs.dev/foundation/internal/findroot"
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
	gomodDir, err := findroot.Find("go module", srcPath, findroot.LookForFile("go.mod"))
	if err != nil {
		return nil, "", err
	}

	gomodFile := filepath.Join(gomodDir, "go.mod")
	gomodBytes, err := os.ReadFile(gomodFile)
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
