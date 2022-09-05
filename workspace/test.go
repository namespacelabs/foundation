// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package workspace

import (
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

func TransformTest(loc pkggraph.Location, test *schema.Test) error {
	if test.PackageName != "" {
		return fnerrors.UserError(loc, "package_name can not be set")
	}

	if test.Name == "" {
		return fnerrors.UserError(loc, "test name must be set")
	}

	if test.Driver == nil {
		return fnerrors.UserError(loc, "driver must be set")
	}

	if test.Driver.Name != "" && test.Driver.Name != test.Name {
		return fnerrors.UserError(loc, "driver.name must be unset or be the same as the test name")
	} else {
		test.Driver.Name = test.Name
	}

	if err := TransformBinary(loc, test.Driver); err != nil {
		return err
	}

	if len(test.ServersUnderTest) == 0 {
		return fnerrors.UserError(loc, "need at least one server under test")
	}

	test.PackageName = loc.PackageName.String()

	return nil
}
