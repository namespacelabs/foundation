// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cuefrontendopaque

import (
	"context"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

type cueIntegration struct {
	cueIntegrationDocker
	cueIntegrationGo

	Kind string `json:"kind"`

	// Shortcuts
	Docker *cueIntegrationDocker `json:"docker"`
	Golang *cueIntegrationGo     `json:"go"`
}

type cueIntegrationDocker struct {
	Dockerfile string `json:"dockerfile"`
}

type cueIntegrationGo struct {
	Package string `json:"pkg"`
}

// Mutates "pkg"
func parseIntegration(ctx context.Context, loc pkggraph.Location, v *fncue.CueV, pkg *pkggraph.Package) error {
	var bits cueIntegration
	if err := v.Val.Decode(&bits); err != nil {
		return err
	}

	// Parsing shortcuts
	if bits.Kind == "" {
		if bits.Docker != nil {
			bits.cueIntegrationDocker = *bits.Docker
			bits.Kind = serverKindDockerfile
		}
		if bits.Golang != nil {
			bits.cueIntegrationGo = *bits.Golang
			bits.Kind = serverKindGo
		}
	}

	var bp *schema.ImageBuildPlan

	switch bits.Kind {
	case serverKindDockerfile:
		if bits.Dockerfile == "" {
			return fnerrors.UserError(loc, "docker integration requires dockerfile")
		}

		bp = &schema.ImageBuildPlan{Dockerfile: bits.cueIntegrationDocker.Dockerfile}
	case serverKindGo:
		goPkg := bits.Package
		if goPkg == "" {
			goPkg = "."
		}

		bp = &schema.ImageBuildPlan{GoPackage: goPkg}
	default:
		return fnerrors.UserError(loc, "unsupported integration kind %q", bits.Kind)
	}

	pkg.Binaries = append(pkg.Binaries, &schema.Binary{
		Name: pkg.Server.Name,
		BuildPlan: &schema.LayeredImageBuildPlan{
			LayerBuildPlan: []*schema.ImageBuildPlan{bp},
		},
	})
	pkg.Server.Binary = &schema.Server_Binary{
		PackageRef: schema.MakePackageRef(loc.PackageName, pkg.Server.Name),
	}

	return nil
}

// Mutates "pkg"
func parseImageIntegration(ctx context.Context, loc pkggraph.Location, v *fncue.CueV, pkg *pkggraph.Package) error {
	str, err := v.Val.String()
	if err != nil {
		return err
	}

	pkg.Binaries = append(pkg.Binaries, &schema.Binary{
		Name: pkg.Server.Name,
		BuildPlan: &schema.LayeredImageBuildPlan{
			LayerBuildPlan: []*schema.ImageBuildPlan{
				{ImageId: str},
			},
		},
	})

	pkg.Server.Binary = &schema.Server_Binary{
		PackageRef: schema.MakePackageRef(loc.PackageName, pkg.Server.Name),
	}

	return nil
}
