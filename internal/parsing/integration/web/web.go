// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package web

import (
	"context"
	"fmt"
	"path/filepath"

	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/parsing/integration/api"
	"namespacelabs.dev/foundation/internal/parsing/integration/nodejs"
	"namespacelabs.dev/foundation/internal/protos"
	"namespacelabs.dev/foundation/languages/nodejs/binary"
	"namespacelabs.dev/foundation/languages/opaque"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/std/runtime"
)

const (
	runtimePkg = "namespacelabs.dev/foundation/std/runtime"
)

func Apply(ctx context.Context, env *schema.Environment, pl pkggraph.PackageLoader, data *schema.WebIntegration, pkg *pkggraph.Package) error {
	if pkg.Server == nil {
		// Can't happen with the current syntax.
		return fnerrors.UserError(pkg.Location, "web integration requires a server")
	}

	pkg.Server.Framework = schema.Framework_OPAQUE_NODEJS

	// Adding a dependency to the backends via resources.
	if len(data.Nodejs.Backend) > 0 {
		if err := injectBackendsAsResourceDeps(ctx, pl, pkg, data.Nodejs.Backend); err != nil {
			return err
		}
	}

	var port int32
	for _, s := range append(pkg.Server.Service, pkg.Server.Ingress...) {
		if s.Name == data.Service {
			port = s.Port.ContainerPort
			break
		}
	}

	if port == 0 {
		return fnerrors.UserError(pkg.Location, "web integration: couldn't find service %q", data.Service)
	}

	binaryRef, err := api.GenerateBinaryAndAddToPackage(ctx, env, pl, pkg, pkg.Server.Name, &schema.WebIntegration_Build{
		Nodejs:         data.Nodejs,
		BuildOutputDir: data.BuildOutputDir,
		Port:           port,
	})
	if err != nil {
		return err
	}

	return api.SetServerBinaryRef(pkg, binaryRef)
}

func injectBackendsAsResourceDeps(ctx context.Context, pl pkggraph.PackageLoader, pkg *pkggraph.Package, backends []*schema.NodejsIntegration_Backend) error {
	if pkg.Server.ResourcePack == nil {
		pkg.Server.ResourcePack = &schema.ResourcePack{}
	}

	// Making sure that the runtime package is loaded.
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

func CreateBinary(ctx context.Context, env *schema.Environment, pl pkggraph.PackageLoader, loc pkggraph.Location, data *schema.WebIntegration_Build) (*schema.Binary, error) {
	nodejsData := protos.Clone(data.Nodejs)
	nodejsData.BuildOutputDir = data.BuildOutputDir
	nodejsBinary, err := nodejs.CreateBinary(ctx, env, pl, loc, nodejsData)
	if err != nil {
		return nil, err
	}

	if opaque.UseDevBuild(env) {
		return nodejsBinary, nil
	} else {
		return &schema.Binary{
			BuildPlan: &schema.LayeredImageBuildPlan{
				LayerBuildPlan: append(
					[]*schema.ImageBuildPlan{{
						Description: "nginx",
						StaticFilesServer: &schema.ImageBuildPlan_StaticFilesServer{
							Dir:  filepath.Join(binary.AppRootPath, data.BuildOutputDir),
							Port: data.Port,
						}}},
					nodejsBinary.BuildPlan.LayerBuildPlan...,
				),
			},
			Config: &schema.BinaryConfig{
				Command: []string{"nginx"},
				Args:    []string{"-g", "daemon off;"},
			},
		}, nil
	}
}
