// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package workspace

import (
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

func TransformBinary(loc pkggraph.Location, bin *schema.Binary) error {
	if bin.PackageName != "" {
		return fnerrors.UserError(loc, "package_name can not be set")
	}

	if bin.Name == "" {
		return fnerrors.UserError(loc, "binary name can't be empty")
	}

	if bin.BuildPlan != nil && bin.From != nil {
		return fnerrors.UserError(loc, "binary.build_plan and binary.from are exclusive and can't be both be set")
	}

	bin.PackageName = loc.PackageName.String()

	if bin.Config == nil {
		// By default, assume the binary is built with the same name as the package name.
		bin.Config = &schema.BinaryConfig{
			Command: []string{"/" + bin.Name},
		}
	}

	return nil
}

func TransformFunction(loc pkggraph.Location, function *schema.ExperimentalFunction) error {
	if function.PackageName != "" {
		return fnerrors.UserError(loc, "package_name can not be set")
	}

	function.PackageName = loc.PackageName.String()

	if function.Kind == "" {
		return fnerrors.UserError(loc, "function kind can't be empty")
	}

	if function.Runtime == "" {
		return fnerrors.UserError(loc, "function runtime can't be empty")
	}

	if function.Source == "" {
		return fnerrors.UserError(loc, "function source must be specified")
	}

	return nil
}
