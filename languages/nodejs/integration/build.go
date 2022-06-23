// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package integration

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
	"namespacelabs.dev/foundation/build/binary"
	"namespacelabs.dev/foundation/build/buildkit"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs/memfs"
	"namespacelabs.dev/foundation/internal/llbutil"
	"namespacelabs.dev/foundation/internal/production"
	"namespacelabs.dev/foundation/internal/sdk/yarn"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/devhost"
	"namespacelabs.dev/foundation/workspace/pins"
)

const appRootPath = "/app"

// These paths are only used within a buildkit environment.
var (
	// All dependencies that are not from the same module copied here. This includes
	// dependencies used as "Dep" in workspace (copied from the Namespace cache)
	// and the ones used as "Replace" (copied from the user's file system).
	// We add "node_modules" so Yarn doesn't recognize external modules as workspaces.
	depsRootPath      = filepath.Join(appRootPath, "external_deps", "node_modules")
	yarnBinaryPath    = "/yarn.cjs"
	tsConfigPath      = filepath.Join(appRootPath, "tsconfig.production.fn.json")
	nodemonConfigPath = filepath.Join(appRootPath, "nodemon.fn.json")
	lockFilePath      = "/fn.lock.json"
)

type buildNodeJS struct {
	module          build.Workspace
	workspace       *schema.Workspace
	externalModules []build.Workspace
	yarnRoot        string
	serverEnv       provision.ServerEnv
	isDevBuild      bool
	isFocus         bool
}

func (bnj buildNodeJS) BuildImage(ctx context.Context, env ops.Environment, conf build.Configuration) (compute.Computable[oci.Image], error) {
	nodeImage, err := pins.CheckDefault("node")
	if err != nil {
		return nil, err
	}

	n := NodeJsBinary{
		NodeJsBase: nodeImage,
		Env:        nodeEnv(env),
	}

	state, local, err := n.LLB(ctx, bnj, conf)
	if err != nil {
		return nil, err
	}

	nodejsImage, err := buildkit.LLBToImage(ctx, env, conf, state, local...)
	if err != nil {
		return nil, err
	}

	if bnj.isDevBuild {
		// Adding dev controller
		pkg, err := bnj.serverEnv.LoadByName(ctx, controllerPkg)
		if err != nil {
			return nil, err
		}

		p, err := binary.Plan(ctx, pkg, binary.BuildImageOpts{UsePrebuilts: true})
		if err != nil {
			return nil, err
		}

		devControllerImage, err := p.Plan.Spec.BuildImage(ctx, env,
			build.NewBuildTarget(conf.TargetPlatform()).
				WithTargetName(conf.PublishName()).
				WithSourceLabel(p.Plan.SourceLabel).
				WithWorkspace(p.Plan.Workspace))
		if err != nil {
			return nil, err
		}

		images := []compute.Computable[oci.Image]{nodejsImage, devControllerImage}

		return oci.MergeImageLayers(images...), nil
	}

	return nodejsImage, nil
}

func nodeEnv(env ops.Environment) string {
	if env.Proto().GetPurpose() == schema.Environment_PRODUCTION {
		return "production"
	} else {
		return "development"
	}
}

func (buildNodeJS) PlatformIndependent() bool { return false }

type NodeJsBinary struct {
	NodeJsBase string
	Env        string
}

func (n NodeJsBinary) LLB(ctx context.Context, bnj buildNodeJS, conf build.Configuration) (llb.State, []buildkit.LocalContents, error) {
	local := buildkit.LocalContents{Module: bnj.module, Path: ".", ObserveChanges: bnj.isFocus}
	src := buildkit.MakeLocalState(local)

	locals := []buildkit.LocalContents{local}

	yarnRoot := filepath.Join(appRootPath, bnj.yarnRoot)
	buildBase, err := prepareYarnNodejsBase(ctx, n.NodeJsBase, *conf.TargetPlatform(), bnj.isDevBuild)
	if err != nil {
		return llb.State{}, nil, err
	}

	lockFileStruct, err := generateLockFileStruct(bnj.workspace, appRootPath)
	if err != nil {
		return llb.State{}, nil, err
	}

	// When building an image we simply put all the dependencies under "depsRootPath" by their module name.
	for moduleName, module := range lockFileStruct.Modules {
		if module.Path != appRootPath {
			lockFileStruct.Modules[moduleName] = lockFileModule{
				Path: filepath.Join(depsRootPath, moduleName),
			}
		}
	}
	buildBase, err = writeJsonAsFile(ctx, buildBase, lockFileStruct, lockFilePath)
	if err != nil {
		return llb.State{}, nil, err
	}

	// We have to copy the whole Yarn root because otherwise there may be missing workspaces
	// and `yarn install --immutable` will fail.
	buildBase = buildBase.With(llbutil.CopyFrom(src, bnj.yarnRoot, yarnRoot))
	buildBase, err = generateTsConfig(ctx, buildBase, bnj.externalModules, bnj.workspace.ModuleName, yarnRoot)
	if err != nil {
		return llb.State{}, nil, err
	}
	buildBase, err = generateNodemonConfig(ctx, buildBase)
	if err != nil {
		return llb.State{}, nil, err
	}
	for _, module := range bnj.externalModules {
		// Other modules live outside of this workspace.
		// Copying them from their location to the corresponding place in "depsRootPath".
		if module.ModuleName() != bnj.module.ModuleName() {
			lfModule, ok := lockFileStruct.Modules[module.ModuleName()]
			if !ok {
				return llb.State{}, nil, fnerrors.InternalError("module %s not found in the Namespace lock file", module.ModuleName())
			}

			moduleLocal := buildkit.LocalContents{Module: module, Path: ".", ObserveChanges: false}
			locals = append(locals, moduleLocal)
			buildBase = buildBase.With(llbutil.CopyFrom(buildkit.MakeLocalState(moduleLocal), ".", lfModule.Path))
		}
	}
	buildBase = runYarnInstall(*conf.TargetPlatform(), buildBase, yarnRoot, bnj.isDevBuild)

	var out llb.State
	// The dev and prod builds are different:
	//  - For prod we produce the smallest image, without Yarn and its dependencies.
	//  - For dev we keep the base image with Yarn and install nodemon there.
	// This can cause discrepancies between environments however the risk seems to be small.
	if bnj.isDevBuild {
		out = buildBase
	} else {
		// For non-dev builds creating an optimized, small image.
		// buildBase and prodBase must have compatible libcs, e.g. both must be glibc or musl.
		out = production.PrepareImage(llbutil.Image(n.NodeJsBase, *conf.TargetPlatform()), *conf.TargetPlatform()).
			With(llbutil.CopyFrom(buildBase, appRootPath, appRootPath))
	}

	out = out.AddEnv("NODE_ENV", n.Env)

	return out, locals, nil
}

func PrepareYarnBase(ctx context.Context, nodejsBase string, platform specs.Platform) (llb.State, error) {
	base := llbutil.Image(nodejsBase, platform)
	buildBase := base.Run(llb.Shlex("apk add --no-cache python2 make g++")).
		Root().
		AddEnv("YARN_CACHE_FOLDER", "/cache/yarn")
	for k, v := range YarnEnvArgs("/") {
		buildBase = buildBase.AddEnv(k, v)
	}
	buildBase = buildBase.AddEnv(FnYarnLockEnvVar, lockFilePath)

	buildBase, err := copyYarnBinaryFromCache(ctx, buildBase)
	if err != nil {
		return llb.State{}, err
	}

	buildBase, err = copyYarnAuxFilesFromCache(ctx, buildBase)
	if err != nil {
		return llb.State{}, err
	}

	return buildBase, nil
}

func prepareYarnNodejsBase(ctx context.Context, nodejsBase string, platform specs.Platform, isDevBuild bool) (llb.State, error) {
	buildBase, err := PrepareYarnBase(ctx, nodejsBase, platform)
	if err != nil {
		return llb.State{}, err
	}

	buildBase = buildBase.Run(llb.Shlex(fmt.Sprintf(
		"yarn global add typescript@%s",
		builtin().DevDependencies["typescript"],
	))).Root()

	if isDevBuild {
		// Nodemon is used to watch for changes in the source code within a container and restart the "ts-node" server.
		buildBase = buildBase.Run(llb.Shlex(fmt.Sprintf(
			"yarn global add nodemon@%s ts-node@%s",
			builtin().DevBuildDependencies["nodemon"],
			builtin().DevBuildDependencies["ts-node"],
		))).Root()
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

func copyYarnAuxFilesFromCache(ctx context.Context, base llb.State) (llb.State, error) {
	state, err := llbutil.WriteFS(ctx, yarn.YarnAuxFiles(), base, ".")
	if err != nil {
		return llb.State{}, err
	}

	return state, nil
}

func writeJsonAsFile(ctx context.Context, base llb.State, content any, path string) (llb.State, error) {
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

type tsConfig struct {
	CompilerOptions *tsConfigCompilerOptions `json:"compilerOptions,omitempty"`
	Extends         string                   `json:"extends,omitempty"`
	Include         []string                 `json:"include,omitempty"`
	Exclude         []string                 `json:"exclude,omitempty"`
	TsNode          *tsConfigTsNode          `json:"ts-node,omitempty"`
}

type tsConfigTsNode struct {
	Ignore []string `json:"ignore,omitempty"`
}

type tsConfigCompilerOptions struct {
	SourceMap bool `json:"sourceMap,omitempty"`
}

func generateTsConfig(ctx context.Context, base llb.State, externalModules []build.Workspace, rootModuleName string, yarnRoot string) (llb.State, error) {
	tsConfig := tsConfig{
		CompilerOptions: &tsConfigCompilerOptions{SourceMap: true},
		// tsconfig.json exists as it is generated by "fn generate" if the user create it themselves.
		Extends: filepath.Join(yarnRoot, "./tsconfig.json"),
		Include: []string{"."},
		// These Web targets have Web-only dependencies that are not captured in the root yarn.lock.
		// Hack: excluding them from compilation explicitly.
		// TODO: find a better way to handle Web nodes under a Node.js Yarn root.
		Exclude: []string{"**/devworkflow/web", "**/languages/nodejs/yarnplugin"},
		TsNode: &tsConfigTsNode{
			// By default it ignores node_modules but we need to compile foundation-managed dependencies inside,
			// so changing "ignore" to a pattern that doesn't match anything.
			Ignore: []string{"(?!.*)"},
		},
	}

	for _, module := range externalModules {
		tsConfig.Include = append(tsConfig.Include, fmt.Sprintf("%s/%s", depsRootPath, module.ModuleName()))
	}

	return writeJsonAsFile(ctx, base, tsConfig, tsConfigPath)
}

type nodemonConfig struct {
	ExecMap *nodemonConfigExecMap `json:"execMap,omitempty"`
}

type nodemonConfigExecMap struct {
	Ts []string `json:"ts"`
}

func generateNodemonConfig(ctx context.Context, base llb.State) (llb.State, error) {
	config := nodemonConfig{
		ExecMap: &nodemonConfigExecMap{
			Ts: []string{fmt.Sprintf("ts-node --project %s", tsConfigPath)},
		},
	}

	return writeJsonAsFile(ctx, base, config, nodemonConfigPath)
}

func runYarnInstall(platform specs.Platform, buildBase llb.State, yarnRoot string, isDevBuild bool) llb.State {
	yarnInstall := buildBase.
		Run(RunYarnShlex("install", "--immutable"), llb.Dir(yarnRoot))
	yarnInstall.AddMount("/cache/yarn", llb.Scratch(), llb.AsPersistentCacheDir(
		"yarn-cache-"+strings.ReplaceAll(devhost.FormatPlatform(platform), "/", "-"), llb.CacheMountShared))

	out := yarnInstall.Root()

	// No need to compile Typescript for dev builds, "nodemon" does it itself.
	if !isDevBuild {
		out = out.Run(RunYarnShlex("tsc", "--project", tsConfigPath), llb.Dir(yarnRoot)).Root()
	}

	return out
}

func RunYarnShlex(args ...string) llb.RunOption {
	return llb.Shlex(fmt.Sprintf("node %s %s", yarnBinaryPath, strings.Join(args, " ")))
}
