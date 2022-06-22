// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package integration

import (
	"encoding/json"
	"path/filepath"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/dirs"
)

const (
	lockFn = "fn.lock.json"
)

type lockFile struct {
	Modules map[string]lockFileModule `json:"modules"`
}

type lockFileModule struct {
	Path string `json:"path"`
}

func generateLockFileStruct(workspace *schema.Workspace, moduleAbsPath string) (lockFile, error) {
	moduleCacheRoot, err := dirs.ModuleCacheRoot()
	if err != nil {
		return lockFile{}, err
	}

	lock := lockFile{
		Modules: map[string]lockFileModule{},
	}

	for _, dep := range workspace.Dep {
		lock.Modules[dep.ModuleName] = lockFileModule{
			Path: filepath.Join(moduleCacheRoot, dep.ModuleName, dep.Version),
		}
	}

	for _, replace := range workspace.Replace {
		lock.Modules[replace.ModuleName] = lockFileModule{
			Path: filepath.Join(moduleAbsPath, replace.Path),
		}
	}

	// The module itself is needed to resolve dependencies between nodes within the module.
	lock.Modules[workspace.ModuleName] = lockFileModule{
		Path: moduleAbsPath,
	}

	return lock, nil
}

// Returns the filename
func writeLockFileToTemp(workspaceData workspace.WorkspaceData) (string, error) {
	lockStruct, err := generateLockFileStruct(workspaceData.Parsed(), workspaceData.AbsPath())
	if err != nil {
		return "", err
	}

	lock, err := json.MarshalIndent(lockStruct, "", "\t")
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
