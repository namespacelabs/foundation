// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package nodejs

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/moby/buildkit/client/llb"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/build"
	"namespacelabs.dev/foundation/build/buildkit"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs/memfs"
	"namespacelabs.dev/foundation/internal/llbutil"
	"namespacelabs.dev/foundation/internal/sdk/yarn"
	"namespacelabs.dev/foundation/languages/nodejs/yarnplugin"
	"namespacelabs.dev/foundation/schema"
)

const (
	yarnLockFilePath      = "/fn.lock.json"
	YarnContainerCacheDir = "/cache/yarn"
	yarnRcContent         = `nodeLinker: node-modules

enableTelemetry: false

logFilters:
  - code: YN0013
    level: discard
  - code: YN0072
    level: discard
`
	// All dependencies that are not from the same module copied here. This includes
	// dependencies used as "Dep" in workspace (copied from the Namespace cache)
	// and the ones used as "Replace" (copied from the user's file system).
	// TODO: figure out why tsc fails if it is not under "/app"
	DepsRootPath = "/app/external_deps"
)

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
		AddEnv(fnYarnLockEnvVar, yarnLockFilePath)
	for k, v := range yarnEnvArgs("/") {
		buildBase = buildBase.AddEnv(k, v)
	}
	buildBase = buildBase.AddEnv(fnYarnLockEnvVar, yarnLockFilePath)

	return buildBase, nil
}

func AddExternalModules(ctx context.Context, workspace *schema.Workspace, module build.Workspace, rel string, rebuildOnChanges bool, base llb.State, externalModules []build.Workspace) ([]buildkit.LocalContents, llb.State, error) {
	local := buildkit.LocalContents{Module: module, Path: rel, ObserveChanges: rebuildOnChanges}

	locals := []buildkit.LocalContents{local}

	lockFileStruct := generateLockFileStructForBuild(workspace)

	buildBase, err := WriteJsonAsFile(ctx, base, lockFileStruct, yarnLockFilePath)
	if err != nil {
		return nil, llb.State{}, err
	}

	for _, m := range externalModules {
		// Copying external modules to "DepsRootPath".
		lfModule, ok := lockFileStruct.Modules[m.ModuleName()]
		if !ok {
			return nil, llb.State{}, fnerrors.InternalError("module %s not found in the Namespace lock file", module.ModuleName())
		}

		moduleLocal := buildkit.LocalContents{Module: m, Path: ".", ObserveChanges: false}
		locals = append(locals, moduleLocal)
		buildBase = buildBase.With(llbutil.CopyFrom(buildkit.MakeLocalState(moduleLocal), ".", lfModule.Path))
	}

	return locals, buildBase, nil
}

// TODO: move elsewhere
func WriteJsonAsFile(ctx context.Context, base llb.State, content any, path string) (llb.State, error) {
	base = base.File(llb.Mkdir(filepath.Dir(path), 0755, llb.WithParents(true)))
	json, err := json.MarshalIndent(content, "", "\t")
	if err != nil {
		return llb.State{}, err
	}
	var fsys memfs.FS
	fsys.Add(path, json)
	state, err := llbutil.WriteFS(ctx, &fsys, base, ".")
	if err != nil {
		return llb.State{}, err
	}

	return state, nil
}
