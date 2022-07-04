// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package nodejs

import (
	"encoding/json"
	"path/filepath"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/dirs"
)

const (
	lockFn = "fn.lock.json"
)

type lockFile struct {
	RootPath string                    `json:"rootPath"`
	Modules  map[string]LockFileModule `json:"modules"`
}

type LockFileModule struct {
	Path string `json:"path"`
}

func GenerateLockFileStruct(workspace *schema.Workspace, moduleAbsPath string) (lockFile, error) {
	moduleCacheRoot, err := dirs.ModuleCacheRoot()
	if err != nil {
		return lockFile{}, err
	}

	lock := lockFile{
		RootPath: moduleAbsPath,
		Modules:  map[string]LockFileModule{},
	}

	for _, dep := range workspace.Dep {
		moduleRelPath, err := filepath.Rel(moduleAbsPath, filepath.Join(moduleCacheRoot, dep.ModuleName, dep.Version))
		if err != nil {
			return lockFile{}, err
		}

		lock.Modules[dep.ModuleName] = LockFileModule{
			Path: moduleRelPath,
		}
	}

	for _, replace := range workspace.Replace {
		lock.Modules[replace.ModuleName] = LockFileModule{
			Path: replace.Path,
		}
	}

	// The module itself is needed to resolve dependencies between nodes within the module.
	lock.Modules[workspace.ModuleName] = LockFileModule{
		Path: ".",
	}

	return lock, nil
}

// Returns the filename
func writeLockFileToTemp(lockFileStruct lockFile) (string, error) {
	lock, err := json.MarshalIndent(lockFileStruct, "", "\t")
	if err != nil {
		return "", err
	}

	file, err := dirs.CreateUserTemp("nodejs", lockFn)
	if err != nil {
		return "", fnerrors.InternalError("failed to create the %s file: %w", lockFn, err)
	}

	_, err = file.Write(lock)
	return file.Name(), err
}
