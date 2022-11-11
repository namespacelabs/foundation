// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package binary

import (
	"context"

	"namespacelabs.dev/foundation/internal/fnerrors"
	integrationparsing "namespacelabs.dev/foundation/internal/frontend/cuefrontend/integration/api"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/internal/parsing"
	integrationapplying "namespacelabs.dev/foundation/internal/parsing/integration/api"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

type ParseImageOpts struct {
	Required bool
}

// Parses "image"/"imageFrom" fields.
// If needed, generates a binary with the given name and adds it to the package.
// Returns nil if neither of the fields is present.
func ParseImage(ctx context.Context, env *schema.Environment, pl parsing.EarlyPackageLoader, pkg *pkggraph.Package, binaryName string, v *fncue.CueV, opts ParseImageOpts) (*schema.PackageRef, error) {
	var outRef *schema.PackageRef

	if image := v.LookupPath("image"); image.Exists() {
		str, err := image.Val.String()
		if err != nil {
			return nil, err
		}

		inlineBinary := &schema.Binary{
			Name: binaryName,
			BuildPlan: &schema.LayeredImageBuildPlan{
				LayerBuildPlan: []*schema.ImageBuildPlan{{ImageId: str}},
			},
		}
		outRef = schema.MakePackageRef(pkg.PackageName(), binaryName)

		pkg.Binaries = append(pkg.Binaries, inlineBinary)
	}

	if build := v.LookupPath("imageFrom"); build.Exists() {
		if outRef != nil {
			return nil, fnerrors.NewWithLocation(pkg.Location, "cannot specify both 'imageFrom' and 'image'")
		}

		if binary := build.LookupPath("binary"); binary.Exists() {
			str, err := binary.Val.String()
			if err != nil {
				return nil, err
			}
			outRef, err = schema.ParsePackageRef(pkg.PackageName(), str)
			if err != nil {
				return nil, fnerrors.NewWithLocation(pkg.Location, "parsing binary reference: %w", err)
			}
		} else {
			integration, err := integrationparsing.BuildParser.ParseEntity(ctx, env, pl, pkg.Location, build)
			if err != nil {
				return nil, err
			}

			outRef, err = integrationapplying.GenerateBinaryAndAddToPackage(ctx, env, pl, pkg, binaryName, integration.Data)
			if err != nil {
				return nil, err
			}
		}
	}

	if outRef == nil && opts.Required {
		return nil, fnerrors.NewWithLocation(pkg.Location, "missing 'imageFrom' or 'image' definition")
	}

	return outRef, nil
}
