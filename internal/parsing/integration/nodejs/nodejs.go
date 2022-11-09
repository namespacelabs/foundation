// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package nodejs

import (
	"context"
	"fmt"

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
	runtimePkg = "namespacelabs.dev/foundation/library/runtime"
)

func Register() {
	api.RegisterIntegration[*schema.NodejsBuild, *schema.NodejsBuild](impl{})
}

type impl struct {
	api.DefaultBinaryTestIntegration[*schema.NodejsBuild]
}

func (impl) ApplyToServer(ctx context.Context, env *schema.Environment, pl pkggraph.PackageLoader, pkg *pkggraph.Package, data *schema.NodejsBuild) error {
	if pkg.Server == nil {
		// Can't happen with the current syntax.
		return fnerrors.UserError(pkg.Location, "nodejs integration requires a server")
	}

	pkg.Server.Framework = schema.Framework_OPAQUE_NODEJS

	// Adding a dependency to the backends via resources.
	if len(data.InternalDoNotUseBackend) > 0 {
		if err := InjectBackendsAsResourceDeps(ctx, pl, pkg, data.InternalDoNotUseBackend); err != nil {
			return err
		}
	}

	binaryRef, err := api.GenerateBinaryAndAddToPackage(ctx, env, pl, pkg, pkg.Server.Name, data)
	if err != nil {
		return err
	}

	return api.SetServerBinaryRef(pkg, binaryRef)
}

func (impl) CreateBinary(ctx context.Context, env *schema.Environment, pl pkggraph.PackageLoader, loc pkggraph.Location, data *schema.NodejsBuild) (*schema.Binary, error) {
	return CreateNodejsBinary(ctx, env, pl, loc, data)
}

func CreateNodejsBinary(ctx context.Context, env *schema.Environment, pl pkggraph.PackageLoader, loc pkggraph.Location, data *schema.NodejsBuild) (*schema.Binary, error) {
	layers := []*schema.ImageBuildPlan{{
		Description: loc.PackageName.String(),
		NodejsBuild: data,
	}}

	packageManager, err := binary.LookupPackageManager(data.NodePkgMgr)
	if err != nil {
		return nil, err
	}

	config := &schema.BinaryConfig{
		WorkingDir: binary.AppRootPath,
		Command:    []string{packageManager.CLI},
		Env: []*schema.BinaryConfig_EnvEntry{
			{Name: "NODE_ENV", Value: binary.NodeEnv(env)},
		},
	}

	if opaque.UseDevBuild(env) {
		// Making sure that the controller package is loaded.
		_, err := pl.LoadByName(ctx, hotreload.ControllerPkg.AsPackageName())
		if err != nil {
			return nil, err
		}

		layers = append(layers, &schema.ImageBuildPlan{
			Description: hotreload.ControllerPkg.PackageName,
			Binary:      hotreload.ControllerPkg,
		})

		config.Command = []string{hotreload.ControllerCommand}
		// Existence of the "dev" script is not checked, because this code is executed during package loading,
		// and for "ns test" it happens initially with the "DEV" environment.
		config.Args = []string{binary.AppRootPath, fmt.Sprint(hotreload.FileSyncPort), packageManager.CLI, "run", data.RunScript}
	} else {
		config.Args = []string{"run", data.RunScript}
	}

	return &schema.Binary{
		BuildPlan: &schema.LayeredImageBuildPlan{LayerBuildPlan: layers},
		Config:    config,
	}, nil
}

func InjectBackendsAsResourceDeps(ctx context.Context, pl pkggraph.PackageLoader, pkg *pkggraph.Package, backends []*schema.NodejsBuild_Backend) error {
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
