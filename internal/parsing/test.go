// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package parsing

import (
	"context"

	"golang.org/x/exp/slices"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

const StartupTestBinary schema.PackageName = "namespacelabs.dev/foundation/std/startup/testdriver"

func transformTest(loc pkggraph.Location, server *schema.Server, test *schema.Test) error {
	if test.PackageName != "" {
		return fnerrors.NewWithLocation(loc, "package_name can not be set")
	}

	if test.Name == "" {
		return fnerrors.NewWithLocation(loc, "test name must be set")
	}

	if test.Driver == nil {
		return fnerrors.NewWithLocation(loc, "driver must be set")
	}

	if test.Driver.Name != "" && test.Driver.Name != test.Name {
		return fnerrors.NewWithLocation(loc, "driver.name must be unset or be the same as the test name")
	} else {
		test.Driver.Name = test.Name
	}

	if server != nil && !slices.Contains(test.ServersUnderTest, server.PackageName) {
		test.ServersUnderTest = append(test.ServersUnderTest, server.PackageName)
	}

	if len(test.ServersUnderTest) == 0 {
		return fnerrors.NewWithLocation(loc, "need at least one server under test")
	}

	test.PackageName = loc.PackageName.String()

	return nil
}

func createServerStartupTest(ctx context.Context, pl pkggraph.PackageLoader, pkgName schema.PackageName) (*schema.Test, error) {
	return &schema.Test{
		PackageName: pkgName.String(),
		Name:        "startup-test",
		ServersUnderTest: []string{
			pkgName.String(),
		},
		Driver: schema.MakePackageSingleRef(StartupTestBinary),
		Tag:    []string{"generated"},
	}, nil
}
