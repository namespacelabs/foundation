// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package integration

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/moby/buildkit/client/llb"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/build"
	"namespacelabs.dev/foundation/build/binary"
	"namespacelabs.dev/foundation/build/buildkit"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/llbutil"
	"namespacelabs.dev/foundation/internal/nodejs"
	"namespacelabs.dev/foundation/internal/production"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/std/planning"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/devhost"
	"namespacelabs.dev/foundation/workspace/pins"
)

const appRootPath = "/app"

// These paths are only used within a buildkit environment.
var (
	tsConfigPath      = filepath.Join(appRootPath, "tsconfig.production.fn.json")
	nodemonConfigPath = filepath.Join(appRootPath, "nodemon.fn.json")
)

type buildNodeJS struct {
	module          build.Workspace
	workspace       *schema.Workspace
	externalModules []build.Workspace
	yarnRoot        pkggraph.Location
	serverEnv       pkggraph.SealedContext
	isDevBuild      bool
	isFocus         bool
}

func (bnj buildNodeJS) BuildImage(ctx context.Context, env planning.Context, conf build.Configuration) (compute.Computable[oci.Image], error) {
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

		images := []oci.NamedImage{oci.MakeNamedImage(fmt.Sprintf("%s + %s", nodeImage, bnj.module.ModuleName()), nodejsImage), oci.MakeNamedImage(p.Plan.SourceLabel, devControllerImage)}

		return oci.MergeImageLayers(images...), nil
	}

	return nodejsImage, nil
}

func nodeEnv(env planning.Context) string {
	if env.Environment().GetPurpose() == schema.Environment_PRODUCTION {
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
	buildBase, err := prepareYarnNodejsBase(ctx, n.NodeJsBase, *conf.TargetPlatform(), bnj.isDevBuild)
	if err != nil {
		return llb.State{}, nil, err
	}

	local := buildkit.LocalContents{Module: bnj.module, Path: ".", ObserveChanges: bnj.isFocus}

	locals, buildBase, err := nodejs.AddExternalModules(ctx, bnj.workspace, ".", buildBase, bnj.externalModules)
	if err != nil {
		return llb.State{}, nil, err
	}
	locals = append(locals, local)

	yarnRoot := filepath.Join(appRootPath, bnj.yarnRoot.Rel())

	buildBase, err = generateTsConfig(ctx, buildBase, bnj.externalModules, bnj.workspace.ModuleName, yarnRoot)
	if err != nil {
		return llb.State{}, nil, err
	}

	buildBase, err = generateNodemonConfig(ctx, buildBase)
	if err != nil {
		return llb.State{}, nil, err
	}

	src := buildkit.MakeLocalState(local)
	buildBase = buildBase.With(
		llbutil.CopyFrom(src, bnj.yarnRoot.Rel(), yarnRoot),
		yarnInstallAndBuild(*conf.TargetPlatform(), yarnRoot, bnj.isDevBuild))

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
		out = llbutil.Image(n.NodeJsBase, *conf.TargetPlatform()).With(
			production.NonRootUser(),
			llbutil.CopyFrom(buildBase, appRootPath, appRootPath),
		)
	}

	out = out.AddEnv("NODE_ENV", n.Env)

	return out, locals, nil
}

func prepareYarnNodejsBase(ctx context.Context, nodejsBase string, platform specs.Platform, isDevBuild bool) (llb.State, error) {
	buildBase, err := nodejs.PrepareNodejsBaseWithYarnForBuild(ctx, nodejsBase, platform)
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
		Exclude: []string{
			"**/webui",
			"**/devworkflow/web",
			"**/languages/nodejs/yarnplugin",
		},
		TsNode: &tsConfigTsNode{
			// By default it ignores node_modules but we need to compile foundation-managed dependencies inside,
			// so changing "ignore" to a pattern that doesn't match anything.
			Ignore: []string{"(?!.*)"},
		},
	}

	// Skip Deno sources.
	if rootModuleName == "namespacelabs.dev/foundation" {
		tsConfig.Exclude = append(tsConfig.Exclude, "**/std/experimental", "**/std/testdata/datastore/denokeygen")
	}

	for _, module := range externalModules {
		tsConfig.Include = append(tsConfig.Include, fmt.Sprintf("%s/%s", nodejs.DepsRootPath, module.ModuleName()))
	}

	return llbutil.AddSerializedJsonAsFile(base, tsConfigPath, tsConfig)
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

	return llbutil.AddSerializedJsonAsFile(base, nodemonConfigPath, config)
}

func yarnInstallAndBuild(platform specs.Platform, yarnRoot string, isDevBuild bool) func(buildBase llb.State) llb.State {
	return func(buildBase llb.State) llb.State {
		yarnInstall := buildBase.
			Run(nodejs.RunYarnShlex("install", "--immutable"), llb.Dir(yarnRoot))
		yarnInstall.AddMount(nodejs.YarnContainerCacheDir, llb.Scratch(), llb.AsPersistentCacheDir(
			"yarn-cache-"+strings.ReplaceAll(devhost.FormatPlatform(platform), "/", "-"), llb.CacheMountShared))

		out := yarnInstall.Root()

		// No need to compile Typescript for dev builds, "nodemon" does it itself.
		if !isDevBuild {
			out = out.Run(nodejs.RunYarnShlex("tsc", "--project", tsConfigPath), llb.Dir(yarnRoot)).Root()
		}

		return out
	}
}
