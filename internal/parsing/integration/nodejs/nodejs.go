// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package nodejs

import (
	"context"
	"fmt"

	"golang.org/x/exp/slices"
	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/hotreload"
	"namespacelabs.dev/foundation/internal/languages/nodejs/binary"
	"namespacelabs.dev/foundation/internal/languages/opaque"
	"namespacelabs.dev/foundation/internal/parsing/integration/api"
	"namespacelabs.dev/foundation/library/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

const (
	startScript = "start"
	buildScript = "build"
	devScript   = "dev"
	runtimePkg  = "namespacelabs.dev/foundation/library/runtime"
)

func Apply(ctx context.Context, env *schema.Environment, pl pkggraph.PackageLoader, data *schema.NodejsIntegration, pkg *pkggraph.Package) error {
	if pkg.Server == nil {
		// Can't happen with the current syntax.
		return fnerrors.UserError(pkg.Location, "nodejs integration requires a server")
	}

	pkg.Server.Framework = schema.Framework_OPAQUE_NODEJS

	// Adding a dependency to the backends via resources.
	if len(data.Backend) > 0 {
		if err := InjectBackendsAsResourceDeps(ctx, pl, pkg, data.Backend); err != nil {
			return err
		}
	}

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

func InjectBackendsAsResourceDeps(ctx context.Context, pl pkggraph.PackageLoader, pkg *pkggraph.Package, backends []*schema.NodejsIntegration_Backend) error {
	if pkg.Server.ResourcePack == nil {
		pkg.Server.ResourcePack = &schema.ResourcePack{}
	}

	// Must ensure that the server runtime class (ServerIntent) is loaded.
	if _, err := pl.LoadByName(ctx, runtimePkg); err != nil {
		return err
	}

	for _, b := range backends {
		// Making sure that the backend package is loaded.
		if _, err := pl.LoadByName(ctx, b.Service.AsPackageName()); err != nil {
			return err
		}

		intent, err := anypb.New(&runtime.ServerIntent{PackageName: b.Service.PackageName})
		if err != nil {
			return err
		}

		pkg.Server.ResourcePack.ResourceInstance = append(pkg.Server.ResourcePack.ResourceInstance, &schema.ResourceInstance{
			PackageName: string(pkg.PackageName()),
			Name:        fmt.Sprintf("gen-backend-resource-%s", b.Service.Name),
			Class:       schema.MakePackageRef(runtimePkg, "Server"),
			Intent:      intent,
		})
	}

	return nil
}
