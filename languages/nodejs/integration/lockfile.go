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
	lockFilePath       = "node_modules/fn.lock.json"
	foundationModule   = "namespacelabs.dev/foundation"
	runtimePackage     = "namespacelabs/foundation"
	runtimePackagePath = "languages/nodejs/runtime"
)

type lockFile struct {
	ModuleToPath map[string]string `json:"moduleToPath"`
}

func generateLockFileStruct(workspace *schema.Workspace, moduleAbsPath string) (lockFile, error) {
	moduleCacheRoot, err := dirs.ModuleCacheRoot()
	if err != nil {
		return lockFile{}, err
	}

	lock := lockFile{
		ModuleToPath: map[string]string{
			workspace.ModuleName: moduleAbsPath,
		},
	}

	for _, dep := range workspace.Dep {
		lock.ModuleToPath[dep.ModuleName] = filepath.Join(moduleCacheRoot, dep.ModuleName, dep.Version)
	}

	for _, replace := range workspace.Replace {
		lock.ModuleToPath[replace.ModuleName] = filepath.Join(moduleAbsPath, replace.Path)
	}

	_, ok := lock.ModuleToPath[foundationModule]
	if !ok {
		// This is the Foundation module itself.
		lock.ModuleToPath[foundationModule] = moduleAbsPath
	}

	return lock, nil
}

func generateLockFile(workspace *schema.Workspace, moduleAbsPath string) ([]byte, error) {
	lock, err := generateLockFileStruct(workspace, moduleAbsPath)
	if err != nil {
		return nil, err
	}

	return json.MarshalIndent(lock, "", "\t")
}

func writeLockFile(ctx context.Context, workspace *schema.Workspace, module *workspace.Module, yarnRoot string) error {
	lock, err := generateLockFile(workspace, module.Abs())
	if err != nil {
		return err
	}

	return fnfs.WriteWorkspaceFile(ctx, console.Stdout(ctx), module.ReadWriteFS(), filepath.Join(yarnRoot, lockFilePath), func(w io.Writer) error {
		_, err := w.Write(lock)
		return err
	})
}
