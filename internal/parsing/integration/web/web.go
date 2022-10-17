// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package web

import (
	"context"
	"path/filepath"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/parsing/integration/api"
	"namespacelabs.dev/foundation/internal/parsing/integration/nodejs"
	"namespacelabs.dev/foundation/internal/protos"
	"namespacelabs.dev/foundation/languages/nodejs/binary"
	"namespacelabs.dev/foundation/languages/opaque"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

func Apply(ctx context.Context, env *schema.Environment, pl pkggraph.PackageLoader, data *schema.WebIntegration, pkg *pkggraph.Package) error {
	if pkg.Server == nil {
		// Can't happen with the current syntax.
		return fnerrors.UserError(pkg.Location, "web integration requires a server")
	}

	pkg.Server.Framework = schema.Framework_OPAQUE_NODEJS

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
