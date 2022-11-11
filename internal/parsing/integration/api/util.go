// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package api

import (
	"context"

	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/support"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

func SetServerBinary(pkg *pkggraph.Package, buildPlan *schema.LayeredImageBuildPlan, commands []string) error {
	pkg.Binaries = append(pkg.Binaries, &schema.Binary{
		Name:      pkg.Server.Name,
		BuildPlan: buildPlan,
		Config: &schema.BinaryConfig{
			Command: commands,
		},
	})

	return SetServerBinaryRef(pkg, schema.MakePackageRef(pkg.Location.PackageName, pkg.Server.Name))
}

func SetServerBinaryRef(pkg *pkggraph.Package, binaryRef *schema.PackageRef) error {
	if pkg.Server.MainContainer.BinaryRef != nil {
		// TODO: add a more meaningful error message
		return fnerrors.NewWithLocation(pkg.Location, "server binary is set multiple times")
	}

	pkg.Server.MainContainer.BinaryRef = binaryRef

	return nil
}

func SetTestDriver(loc pkggraph.Location, test *schema.Test, driver *schema.Binary) error {
	if test.GetDriver().GetBuildPlan() != nil {
		return fnerrors.AttachLocation(loc,
			fnerrors.InternalError("test driver build plan is set multiple times"))
	}

	if test.GetDriver().GetName() != "" || test.GetDriver().GetPackageName() != "" {
		// TODO improve error message
		return fnerrors.AttachLocation(loc,
			fnerrors.InternalError("test driver is set multiple times"))
	}

	args := test.GetDriver().GetConfig().GetArgs()
	envs := test.GetDriver().GetConfig().GetEnv()

	test.Driver = driver

	if test.Driver.Config == nil {
		test.Driver.Config = &schema.BinaryConfig{}
	}

	test.Driver.Config.Args = append(test.Driver.Config.Args, args...)

	var err error
	test.Driver.Config.Env, err = support.MergeEnvs(test.Driver.Config.Env, envs)

	return err
}

func GenerateBinaryAndAddToPackage(ctx context.Context, env *schema.Environment, pl pkggraph.PackageLoader, pkg *pkggraph.Package, binaryName string, data proto.Message) (*schema.PackageRef, error) {
	binary, err := GenerateBinary(ctx, env, pl, pkg.Location, binaryName, data)
	if err != nil {
		return nil, err
	}

	pkg.Binaries = append(pkg.Binaries, binary)

	return schema.MakePackageRef(pkg.Location.PackageName, binaryName), nil
}

// Simply creates a binary for a test
type DefaultBinaryTestIntegration[ServerData proto.Message] struct{}

func (DefaultBinaryTestIntegration[ServerData]) ApplyToTest(ctx context.Context, env *schema.Environment, pl pkggraph.PackageLoader, pkg *pkggraph.Package, test *schema.Test, data ServerData) error {
	_, err := GenerateBinaryAndAddToPackage(ctx, env, pl, pkg, test.Name, data)
	if err != nil {
		return err
	}

	// TODO: use a PackageRef for the test driver binary instead of adding and then removing it from package binaries.
	if err := SetTestDriver(pkg.Location, test, pkg.Binaries[len(pkg.Binaries)-1]); err != nil {
		return err
	}
	pkg.Binaries = pkg.Binaries[:len(pkg.Binaries)-1]

	return nil

}
