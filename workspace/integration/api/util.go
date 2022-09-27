// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package api

import (
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

func SetServerBinary(pkg *pkggraph.Package, buildPlan *schema.LayeredImageBuildPlan, commands []string) error {
	if pkg.Server.MainContainer.BinaryRef != nil {
		// TODO: add a more meaningful error message
		return fnerrors.UserError(pkg.Location, "server binary is set multiple times")
	}

	pkg.Binaries = append(pkg.Binaries, &schema.Binary{
		Name:      pkg.Server.Name,
		BuildPlan: buildPlan,
		Config: &schema.BinaryConfig{
			Command: commands,
		},
	})
	pkg.Server.MainContainer.BinaryRef = schema.MakePackageRef(pkg.Location.PackageName, pkg.Server.Name)

	return nil
}
