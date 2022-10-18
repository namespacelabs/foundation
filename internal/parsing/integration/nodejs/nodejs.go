// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package nodejs

import (
	"context"
	"fmt"

	"golang.org/x/exp/slices"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/hotreload"
	"namespacelabs.dev/foundation/internal/parsing/integration/api"
	"namespacelabs.dev/foundation/languages/nodejs/binary"
	"namespacelabs.dev/foundation/languages/opaque"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
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

	cliName, err := binary.PackageManagerCLI(data.NodePkgMgr)
	if err != nil {
		return nil, err
	}

	nodejsBuild := &schema.ImageBuildPlan_NodejsBuild{
		RelPath:                 nodePkg,
		NodePkgMgr:              data.NodePkgMgr,
		BuildOutDir:             data.BuildOutputDir,
		InternalDoNotUseBackend: data.Backend,
	}
	if slices.Contains(data.PackageJsonScripts, buildScript) {
		nodejsBuild.BuildScript = buildScript
	}

	layers := []*schema.ImageBuildPlan{{
		Description: loc.PackageName.String(),
		NodejsBuild: nodejsBuild,
	}}

	config := &schema.BinaryConfig{
		WorkingDir: binary.AppRootPath,
		Command:    []string{cliName},
	}

	if opaque.UseDevBuild(env) {
		if !slices.Contains(data.PackageJsonScripts, devScript) {
			return nil, fnerrors.UserError(loc, `package.json must contain a script named '%s': it is invoked when starting the server in "dev" environment`, devScript)
		}

		// Making sure that the controller package is loaded.
		_, err := pl.LoadByName(ctx, hotreload.ControllerPkg.AsPackageName())
		if err != nil {
			return nil, err
		}

		layers = append(layers, &schema.ImageBuildPlan{
			Description: hotreload.ControllerPkg.PackageName,
			Binary:      hotreload.ControllerPkg,
		})

		config.Command = []string{"/filesync-controller"}
		config.Args = []string{binary.AppRootPath, fmt.Sprint(hotreload.FileSyncPort), cliName, "run", devScript}
	} else {
		if !slices.Contains(data.PackageJsonScripts, startScript) {
			return nil, fnerrors.UserError(loc, `package.json must contain a script named '%s': it is invoked when starting the server in non-dev environments`, startScript)
		}

		config.Args = []string{"run", startScript}
	}

	return &schema.Binary{
		BuildPlan: &schema.LayeredImageBuildPlan{LayerBuildPlan: layers},
		Config:    config,
	}, nil
}
