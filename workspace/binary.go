// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package workspace

import (
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
)

func TransformBinary(loc Location, bin *schema.Binary) error {
	if bin.PackageName != "" {
		return fnerrors.UserError(loc, "package_name can not be set")
	}

	if bin.Name == "" {
		return fnerrors.UserError(loc, "binary name can't be empty")
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
