// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package integration

import (
	"context"
	"encoding/json"
	"io"
	"path/filepath"

	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/dirs"
)

const (
	// Must be consistent with the path in the Foundation plugin for Yarn in Typescript.
	lockFilePath     = "node_modules/fn.lock.json"
	foundationModule = "namespacelabs.dev/foundation"
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

	_, ok := lock.Modules[foundationModule]
	if !ok {
		// This is the Foundation module itself.
		lock.Modules[foundationModule] = lockFileModule{
			Path: moduleAbsPath,
		}
	}

	return lock, nil
}

func writeLockFile(ctx context.Context, workspace *schema.Workspace, module *workspace.Module, yarnRoot string) error {
	lockStruct, err := generateLockFileStruct(workspace, module.Abs())
	if err != nil {
		return err
	}

	lock, err := json.MarshalIndent(lockStruct, "", "\t")
	if err != nil {
		return err
	}

	return fnfs.WriteWorkspaceFile(ctx, console.Stdout(ctx), module.ReadWriteFS(), filepath.Join(yarnRoot, lockFilePath), func(w io.Writer) error {
		_, err := w.Write(lock)
		return err
	})
}
