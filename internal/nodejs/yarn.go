// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package nodejs

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/localexec"
	"namespacelabs.dev/foundation/internal/sdk/yarn"
	"namespacelabs.dev/foundation/runtime/rtypes"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/pkggraph"
)

const (
	yarnBinaryPath   = "/yarn.cjs"
	fnYarnLockEnvVar = "FN_YARN_LOCK_FILENAME"
	pluginFn         = "plugin-foundation.cjs"
	yarnRcFn         = ".yarnrc.yml"
)

var UseNativeNode = false

func RunYarn(ctx context.Context, env cfg.Context, loc pkggraph.Location, args []string) error {
	lockFileStruct, err := generateLockFileStruct(loc.Module.Workspace, loc.Module.Abs(), loc.Rel())
	if err != nil {
		return err
	}

	dir, err := os.MkdirTemp("", "ns-yarn")
	if err != nil {
		return err
	}

	if err := writeLockFileToTemp(filepath.Join(dir, lockFn), lockFileStruct); err != nil {
		return err
	}

	yarnFilesDir := "/"
	targetLockDirFn := "/ns-yarn-lock/"
	if UseNativeNode {
		yarnFilesDir = dir
		targetLockDirFn = dir
	}

	envArgs := []*schema.BinaryConfig_EnvEntry{}
	for k, v := range yarnEnvArgs(yarnFilesDir) {
		envArgs = append(envArgs, &schema.BinaryConfig_EnvEntry{Name: k, Value: v})
	}
	envArgs = append(envArgs, &schema.BinaryConfig_EnvEntry{Name: fnYarnLockEnvVar, Value: filepath.Join(targetLockDirFn, lockFn)})

	if UseNativeNode {
		yarnBin, err := yarn.EnsureSDK(ctx)
		if err != nil {
			return err
		}

		if err := writeYarnAuxFiles(ctx, fnfs.ReadWriteLocalFS(dir)); err != nil {
			return err
		}

		var cmd localexec.Command
		cmd.Command = "node"
		for _, kv := range envArgs {
			cmd.AdditionalEnv = append(cmd.AdditionalEnv, fmt.Sprintf("%s=%s", kv.Name, kv.Value))
		}
		cmd.Args = append([]string{string(yarnBin)}, args...)
		cmd.Dir = filepath.Join(env.Workspace().LoadedFrom().AbsPath, loc.Rel())
		return cmd.Run(ctx)
	}

	mounts := []*rtypes.LocalMapping{{HostPath: dir, ContainerPath: targetLockDirFn}}
	for moduleName, module := range lockFileStruct.Modules {
		if moduleName != loc.Module.ModuleName() {
			path := filepath.Join(loc.Module.Abs(), loc.Rel(), module.Path)
			mounts = append(mounts, &rtypes.LocalMapping{
				HostPath:      path,
				ContainerPath: filepath.Join(workspaceContainerDir, path),
			})
		}
	}

	return RunNodejs(ctx, env, loc.Rel(), "node", &RunNodejsOpts{
		Scope:   loc.PackageName,
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
