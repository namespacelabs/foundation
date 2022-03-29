// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package workspace

import (
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
)

func TransformBinary(loc Location, bin *schema.Binary) (*schema.Binary, error) {
	if bin.PackageName != "" {
		return nil, fnerrors.UserError(loc, "package_name can not be set")
	}

	if bin.Name == "" {
		return nil, fnerrors.UserError(loc, "binary name can't be empty")
	}

	bin.PackageName = loc.PackageName.String()
	return bin, nil
}