// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package nodejs

import (
	"encoding/json"
	"os"
	"path/filepath"

	"namespacelabs.dev/foundation/internal/workspace/dirs"
	"namespacelabs.dev/foundation/schema"
)

const (
	lockFn = "fn.lock.json"
)

type lockFile struct {
	Modules map[string]LockFileModule `json:"modules"`
}

type LockFileModule struct {
	Path string `json:"path"`
}

func generateLockFileStruct(workspace *schema.Workspace, moduleAbsPath string, relPath string) (lockFile, error) {
	moduleCacheRoot, err := dirs.ModuleCacheRoot()
	if err != nil {
		return lockFile{}, err
	}

	lock := lockFile{
		Modules: map[string]LockFileModule{},
	}

	for _, dep := range workspace.Dep {
		moduleRelPath, err := filepath.Rel(
			filepath.Join(moduleAbsPath, relPath),
			filepath.Join(moduleCacheRoot, dep.ModuleName, dep.Version))
		if err != nil {
			return lockFile{}, err
		}

		lock.Modules[dep.ModuleName] = LockFileModule{
			Path: moduleRelPath,
		}
	}

	for _, replace := range workspace.Replace {
		moduleRelPath, err := filepath.Rel(
			filepath.Join(moduleAbsPath, relPath),
			filepath.Join(moduleAbsPath, replace.Path))
		if err != nil {
			return lockFile{}, err
		}

		lock.Modules[replace.ModuleName] = LockFileModule{
			Path: moduleRelPath,
		}
	}

	moduleRelPath, err := filepath.Rel(filepath.Join(moduleAbsPath, relPath), moduleAbsPath)
	if err != nil {
		return lockFile{}, err
	}
	// The module itself is needed to resolve dependencies between nodes within the module.
	lock.Modules[workspace.ModuleName] = LockFileModule{
		Path: moduleRelPath,
	}

	return lock, nil
}

func generateLockFileStructForBuild(relPath string, workspace *schema.Workspace) (lockFile, error) {
	lock := lockFile{
		Modules: map[string]LockFileModule{},
	}

	// When building an image we put all the dependencies under "depsRootPath" by their module name.
	for _, moduleName := range workspace.AllReferencedModules() {
		lock.Modules[moduleName] = LockFileModule{
			Path: filepath.Join(DepsRootPath, moduleName),
		}
	}

	moduleRelPath, err := filepath.Rel(relPath, ".")
	if err != nil {
		return lockFile{}, err
	}

	lock.Modules[workspace.ModuleName] = LockFileModule{
		Path: moduleRelPath,
	}

	return lock, nil
}

// Returns the filename
func writeLockFileToTemp(target string, lockFileStruct lockFile) error {
	lock, err := json.MarshalIndent(lockFileStruct, "", "\t")
	if err != nil {
		return err
	}

	return os.WriteFile(target, lock, 0644)
}
