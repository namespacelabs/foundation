// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package nodejs

import (
	"context"

	"golang.org/x/exp/slices"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/languages/nodejs/binary"
	"namespacelabs.dev/foundation/languages/opaque"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/workspace/integration/api"
)

const (
	startScript = "start"
	buildScript = "build"
	devScript   = "dev"
)

func Apply(ctx context.Context, env *schema.Environment, pl pkggraph.PackageLoader, data *schema.NodejsIntegration, pkg *pkggraph.Package) error {
	if pkg.Server == nil {
		// Can't happen with the current syntax.
		return fnerrors.UserError(pkg.Location, "nodejs integration requires a server")
	}

	pkg.Server.Framework = schema.Framework_OPAQUE_NODEJS

	binaryRef, err := api.GenerateBinaryAndAddToPackage(ctx, env, pl, pkg, pkg.Server.Name, data)
	if err != nil {
		return err
	}

	return api.SetServerBinaryRef(pkg, binaryRef)
}

func CreateBinary(ctx context.Context, env *schema.Environment, pl pkggraph.PackageLoader, loc pkggraph.Location, data *schema.NodejsIntegration) (*schema.Binary, error) {
	nodePkg := data.Pkg
	if nodePkg == "" {
		nodePkg = "."
	}

	cliName, err := binary.PkgMgrCliName(data.NodePkgMgr)
	if err != nil {
		return nil, err
	}

	config := &schema.BinaryConfig{
		WorkingDir: binary.AppRootPath,
		Command:    []string{cliName},
	}

	if opaque.UseDevBuild(env) {
		if !slices.Contains(data.PackageJsonScripts, devScript) {
			return nil, fnerrors.UserError(loc, `package.json must contain a script named '%s': it is invoked when starting the server in "dev" environment`, devScript)
		}

		config.Args = []string{"run", devScript}
	} else {
		if !slices.Contains(data.PackageJsonScripts, startScript) {
			return nil, fnerrors.UserError(loc, `package.json must contain a script named '%s': it is invoked when starting the server in non-dev environments`, startScript)
		}

		config.Args = []string{"run", startScript}
	}

	nodejsBuild := &schema.ImageBuildPlan_NodejsBuild{
		RelPath:    nodePkg,
		NodePkgMgr: data.NodePkgMgr,
	}
	if slices.Contains(data.PackageJsonScripts, buildScript) {
		nodejsBuild.BuildScript = buildScript
	}

	return &schema.Binary{
		BuildPlan: &schema.LayeredImageBuildPlan{
			LayerBuildPlan: []*schema.ImageBuildPlan{{NodejsBuild: nodejsBuild}},
		},
		Config: config,
	}, nil
}
