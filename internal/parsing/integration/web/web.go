// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package web

import (
	"context"
	"path/filepath"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/languages/nodejs/binary"
	"namespacelabs.dev/foundation/internal/languages/opaque"
	"namespacelabs.dev/foundation/internal/parsing/integration/api"
	"namespacelabs.dev/foundation/internal/parsing/integration/nodejs"
	"namespacelabs.dev/foundation/internal/protos"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

func Register() {
	api.RegisterIntegration[*schema.WebIntegration, *schema.WebBuild](impl{})
}

type impl struct{}

func (impl) ApplyToServer(ctx context.Context, env *schema.Environment, pl pkggraph.PackageLoader, pkg *pkggraph.Package, data *schema.WebIntegration) error {
	if pkg.Server == nil {
		// Can't happen with the current syntax.
		return fnerrors.UserError(pkg.Location, "web integration requires a server")
	}

	pkg.Server.Framework = schema.Framework_OPAQUE_NODEJS

	// Adding a dependency to the backends via resources.
	if len(data.Nodejs.InternalDoNotUseBackend) > 0 {
		if err := nodejs.InjectBackendsAsResourceDeps(ctx, pl, pkg, data.Nodejs.InternalDoNotUseBackend); err != nil {
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

	binaryRef, err := api.GenerateBinaryAndAddToPackage(ctx, env, pl, pkg, pkg.Server.Name, &schema.WebBuild{
		Nodejs: data.Nodejs,
		Port:   port,
	})
	if err != nil {
		return err
	}

	return api.SetServerBinaryRef(pkg, binaryRef)
}

func (impl) ApplyToTest(ctx context.Context, env *schema.Environment, pl pkggraph.PackageLoader, pkg *pkggraph.Package, test *schema.Test, data *schema.WebIntegration) error {
	return fnerrors.UserError(pkg.Location, "web integration doesn't support tests yet")
}

func (impl) CreateBinary(ctx context.Context, env *schema.Environment, pl pkggraph.PackageLoader, loc pkggraph.Location, data *schema.WebBuild) (*schema.Binary, error) {
	nodejsData := protos.Clone(data.Nodejs)
	nodejsBinary, err := nodejs.CreateNodejsBinary(ctx, env, pl, loc, nodejsData)
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
							Dir:  filepath.Join(binary.AppRootPath, data.Nodejs.Prod.BuildOutDir),
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
