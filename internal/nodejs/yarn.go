// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package nodejs

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/moby/buildkit/client/llb"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/internal/fnfs/memfs"
	"namespacelabs.dev/foundation/internal/llbutil"
	"namespacelabs.dev/foundation/internal/sdk/yarn"
	"namespacelabs.dev/foundation/languages/nodejs/yarnplugin"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/runtime/rtypes"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
)

const (
	YarnLockFilePath      = "/fn.lock.json"
	YarnContainerCacheDir = "/cache/yarn"
	yarnBinaryPath        = "/yarn.cjs"
	fnYarnLockEnvVar      = "FN_YARN_LOCK_FILENAME"
	lockContainerDir      = "/fnyarnlock"
	pluginFn              = "plugin-foundation.cjs"
	yarnRcFn              = ".yarnrc.yml"
	yarnRcContent         = `nodeLinker: node-modules

enableTelemetry: false

logFilters:
  - code: YN0013
    level: discard
`
)

// Runs a configured Yarn.
func RunYarn(ctx context.Context, env provision.Env, relPath string, args []string, workspaceData workspace.WorkspaceData) error {
	lockFn, err := writeLockFileToTemp(workspaceData, workspaceContainerDir)
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

	return RunNodejs(ctx, env, relPath, "node", &RunNodejsOpts{
		Args:    append([]string{string(yarnBinaryPath)}, args...),
		EnvVars: envArgs,
		Mounts:  []*rtypes.LocalMapping{{HostPath: lockDir, ContainerPath: lockContainerDir}},
	})
}

func yarnEnvArgs(root string) map[string]string {
	return map[string]string{
		"YARN_PLUGINS":     filepath.Join(root, pluginFn),
		"YARN_RC_FILENAME": filepath.Join(root, yarnRcFn),
	}
}

func RunYarnShlex(args ...string) llb.RunOption {
	return llb.Shlex(fmt.Sprintf("node %s %s", yarnBinaryPath, strings.Join(args, " ")))
}

func prepareNodejsBaseWithYarn(ctx context.Context, nodejsBase string, platform specs.Platform) (llb.State, error) {
	base := llbutil.Image(nodejsBase, platform)
	buildBase := base.Run(llb.Shlex("apk add --no-cache python2 make g++")).Root()

	buildBase, err := copyYarnBinaryFromCache(ctx, buildBase)
	if err != nil {
		return llb.State{}, err
	}

	buildBase, err = generateYarnAuxFiles(ctx, buildBase)
	if err != nil {
		return llb.State{}, err
	}

	return buildBase, nil
}

func copyYarnBinaryFromCache(ctx context.Context, base llb.State) (llb.State, error) {
	// TODO: feed Yarn SDK as a dependency to the graph to speed up the initial build.
	yarnBin, err := yarn.EnsureSDK(ctx)
	if err != nil {
		return llb.State{}, err
	}
	yarnBinContent, err := os.ReadFile(string(yarnBin))
	if err != nil {
		return llb.State{}, err
	}
	var fsys memfs.FS
	fsys.Add(yarnBinaryPath, yarnBinContent)
	state, err := llbutil.WriteFS(ctx, &fsys, base, ".")
	if err != nil {
		return llb.State{}, err
	}

	return state, nil
}

func generateYarnAuxFiles(ctx context.Context, base llb.State) (llb.State, error) {
	var fsys memfs.FS
	fsys.Add(pluginFn, yarnplugin.PluginContent())
	fsys.Add(yarnRcFn, []byte(yarnRcContent))
	state, err := llbutil.WriteFS(ctx, &fsys, base, ".")
	if err != nil {
		return llb.State{}, err
	}

	return state, nil
}

func PrepareNodejsBaseWithYarnForBuild(ctx context.Context, nodejsBase string, platform specs.Platform) (llb.State, error) {
	buildBase, err := prepareNodejsBaseWithYarn(ctx, nodejsBase, platform)
	if err != nil {
		return llb.State{}, err
	}

	buildBase = buildBase.
		AddEnv("YARN_CACHE_FOLDER", YarnContainerCacheDir).
		AddEnv(fnYarnLockEnvVar, YarnLockFilePath)
	for k, v := range yarnEnvArgs("/") {
		buildBase = buildBase.AddEnv(k, v)
	}
	buildBase = buildBase.AddEnv(fnYarnLockEnvVar, YarnLockFilePath)

	return buildBase, nil
}
