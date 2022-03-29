// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package workspace

import (
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
)

func TransformTest(loc Location, test *schema.Test) (*schema.Test, error) {
	if test.PackageName != "" {
		return nil, fnerrors.UserError(loc, "package_name can not be set")
	}

	if test.Name == "" {
		return nil, fnerrors.UserError(loc, "test name can't be empty")
	}

	if test.Binary.PackageName != "" {
		return nil, fnerrors.UserError(loc, "package_name can not be set")
	}

	if len(test.ServersUnderTest) == 0 {
		return nil, fnerrors.UserError(loc, "need at least one server under test")
	}

	if test.Binary.Name == "" {
		test.Binary.Name = test.Name
	}

	test.PackageName = loc.PackageName.String()
	test.Binary.PackageName = loc.PackageName.String()

	return test, nil
}