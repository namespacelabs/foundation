// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package binary

import (
	"context"
	"strings"

	"namespacelabs.dev/foundation/build"
	"namespacelabs.dev/foundation/build/buildkit"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/fnfs/workspace/wsremote"
	"namespacelabs.dev/foundation/internal/hotreload"
	"namespacelabs.dev/foundation/languages/opaque"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/std/planning"
)

func NodejsBuilder(env planning.Context, loc pkggraph.Location, config *schema.ImageBuildPlan_NodejsBuild, isFocus bool) (build.Spec, error) {
	return &buildNodeJS{
		loc:        loc.Module.MakeLocation(loc.Rel(config.RelPath)),
		config:     config,
		isDevBuild: opaque.UseDevBuild(env.Environment()),
		isFocus:    isFocus,
	}, nil
}

type buildNodeJS struct {
	loc        pkggraph.Location
	config     *schema.ImageBuildPlan_NodejsBuild
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
				for _, p := range pathsForBuild {
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

	nodejsImage, err := buildkit.LLBToImage(ctx, env, conf, state, local...)
	if err != nil {
		return nil, err
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
