// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package nodejs

import (
	"context"
	"path/filepath"

	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/runtime/rtypes"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
)

const (
	yarnBinaryPath   = "/yarn.cjs"
	fnYarnLockEnvVar = "FN_YARN_LOCK_FILENAME"
	lockContainerDir = "/fnyarnlock"
	pluginFn         = "plugin-foundation.cjs"
	yarnRcFn         = ".yarnrc.yml"
)

// Runs a configured Yarn.
func RunYarn(ctx context.Context, env provision.Env, relPath string, args []string, workspaceData workspace.WorkspaceData) error {
	return RunYarnForScope(ctx, env, "", relPath, args, workspaceData)
}

func RunYarnForLocation(ctx context.Context, env provision.Env, loc workspace.Location, args []string, workspaceData workspace.WorkspaceData) error {
	return RunYarnForScope(ctx, env, loc.PackageName, loc.Rel(), args, workspaceData)
}

func RunYarnForScope(ctx context.Context, env provision.Env, scope schema.PackageName, relPath string, args []string, workspaceData workspace.WorkspaceData) error {
	lockFileStruct, err := generateLockFileStruct(
		workspaceData.Parsed(), workspaceData.AbsPath(), relPath)
	if err != nil {
		return err
	}

	lockFn, err := writeLockFileToTemp(lockFileStruct)
	if err != nil {
		return err
	}
	lockBaseFn := filepath.Base(lockFn)
	lockDir := filepath.Dir(lockFn)
	lockContainerFn := filepath.Join(lockContainerDir, lockBaseFn)

	envArgs := []*schema.BinaryConfig_EnvEntry{}
	for k, v := range yarnEnvArgs("/") {
		envArgs = append(envArgs, &schema.BinaryConfig_EnvEntry{Name: k, Value: v})
	}
	envArgs = append(envArgs, &schema.BinaryConfig_EnvEntry{Name: fnYarnLockEnvVar, Value: lockContainerFn})

	mounts := []*rtypes.LocalMapping{{HostPath: lockDir, ContainerPath: lockContainerDir}}
	for moduleName, module := range lockFileStruct.Modules {
		if moduleName != workspaceData.Parsed().ModuleName {
			path := filepath.Join(workspaceData.AbsPath(), relPath, module.Path)
			mounts = append(mounts, &rtypes.LocalMapping{
				HostPath:      path,
				ContainerPath: filepath.Join(workspaceContainerDir, path),
			})
		}
	}

	return RunNodejs(ctx, env, relPath, "node", &RunNodejsOpts{
		Scope:   scope,
		Args:    append([]string{string(yarnBinaryPath)}, args...),
		EnvVars: envArgs,
		Mounts:  mounts,
	})
}

func yarnEnvArgs(root string) map[string]string {
	return map[string]string{
		"YARN_PLUGINS":     filepath.Join(root, pluginFn),
		"YARN_RC_FILENAME": filepath.Join(root, yarnRcFn),
	}
}
