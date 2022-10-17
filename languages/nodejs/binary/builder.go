// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package binary

import (
	"context"
	"strings"

	"namespacelabs.dev/foundation/build"
	"namespacelabs.dev/foundation/build/assets"
	"namespacelabs.dev/foundation/build/buildkit"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/fnfs/workspace/wsremote"
	"namespacelabs.dev/foundation/internal/hotreload"
	"namespacelabs.dev/foundation/languages/opaque"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/pkggraph"
)

func NodejsBuilder(env cfg.Context, loc pkggraph.Location, config *schema.ImageBuildPlan_NodejsBuild, assets assets.AvailableBuildAssets, isFocus bool) (build.Spec, error) {
	return &buildNodeJS{
		loc:        loc.Module.MakeLocation(loc.Rel(config.RelPath)),
		config:     config,
		assets:     assets,
		isDevBuild: opaque.UseDevBuild(env.Environment()),
		isFocus:    isFocus,
	}, nil
}

type buildNodeJS struct {
	loc        pkggraph.Location
	config     *schema.ImageBuildPlan_NodejsBuild
	assets     assets.AvailableBuildAssets
	isDevBuild bool
	isFocus    bool
}

func (buildNodeJS) PlatformIndependent() bool { return false }

func (bnj buildNodeJS) BuildImage(ctx context.Context, env pkggraph.SealedContext, conf build.Configuration) (compute.Computable[oci.Image], error) {
	var module build.Workspace
	if r := wsremote.Ctx(ctx); r != nil && bnj.isFocus && !bnj.loc.Module.IsExternal() && bnj.isDevBuild {
		module = hotreload.NewHotReloadModule(
			bnj.loc.Module,
			// "ModuleName" and "Rel" are empty because we have only one module in the image and
			// we put the package content directly under the root "/app" directory.
			r.For(&wsremote.Signature{ModuleName: "", Rel: ""}),
			func(filepath string) bool {
				for _, p := range packageManagerSources {
					if strings.HasPrefix(filepath, p) {
						return true
					}
				}
				return false
			},
		)
	} else {
		module = bnj.loc.Module
	}

	n := nodeJsBinary{
		nodejsEnv: nodeEnv(env),
		module:    module,
	}

	state, local, err := n.LLB(ctx, bnj, conf)
	if err != nil {
		return nil, err
	}

	return buildkit.BuildImage(ctx, env, conf, state, local...)
}

func nodeEnv(env cfg.Context) string {
	if env.Environment().GetPurpose() == schema.Environment_PRODUCTION {
		return "production"
	} else {
		return "development"
	}
}
