// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package testing

import (
	"context"
	"fmt"

	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/build/assets"
	"namespacelabs.dev/foundation/internal/build/binary"
	"namespacelabs.dev/foundation/internal/build/multiplatform"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/planning"
	"namespacelabs.dev/foundation/internal/planning/deploy"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/internal/support/naming"
	"namespacelabs.dev/foundation/internal/testing/testboot"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/std/runtime/constants"
	"namespacelabs.dev/foundation/std/tasks"
	"namespacelabs.dev/go-ids"
)

type testDriver struct {
	TestRef    *schema.PackageRef
	Location   pkggraph.Location
	Definition *schema.Binary

	Stack *planning.Stack

	SealedContext pkggraph.SealedContext
	Planner       runtime.Planner

	Resources []pkggraph.ResourceInstance
	Debug     bool

	compute.LocalScoped[deploy.PreparedDeployable]
}

func (test *testDriver) Inputs() *compute.In {
	return compute.Inputs().
		Str("testName", test.TestRef.Name).
		Stringer("testPkg", test.TestRef.AsPackageName()).
		Stringer("loc", test.Location).
		Proto("def", test.Definition).
		Proto("stack", test.Stack.Proto()).
		Proto("workspace", test.SealedContext.Workspace().Proto()).
		Proto("env", test.SealedContext.Environment()).
		Indigestible("planner", test.Planner).
		Indigestible("resources", test.Resources).
		Bool("debug", test.Debug)

}

func (driver *testDriver) Action() *tasks.ActionEvent {
	return tasks.Action("test.prepare-driver").Arg("name", driver.TestRef.Name).Arg("package_name", driver.TestRef.AsPackageName())
}

func (driver *testDriver) Compute(ctx context.Context, r compute.Resolved) (deploy.PreparedDeployable, error) {
	platforms, err := driver.Planner.TargetPlatforms(ctx)
	if err != nil {
		return deploy.PreparedDeployable{}, err
	}

	testBin, err := binary.PlanBinary(ctx, driver.SealedContext, driver.SealedContext, driver.Location, driver.Definition, assets.AvailableBuildAssets{}, binary.BuildImageOpts{
		Platforms: platforms,
	})
	if err != nil {
		return deploy.PreparedDeployable{}, err
	}

	registry := driver.Planner.Registry()

	sutServers := driver.Stack.Focus.PackageNamesAsString()
	runtimeConfig, err := deploy.TestStackToRuntimeConfig(&planning.StackWithIngress{Stack: *driver.Stack}, sutServers)
	if err != nil {
		return deploy.PreparedDeployable{}, err
	}

	testReq := &testboot.TestRequest{
		Endpoint:         driver.Stack.Endpoints,
		InternalEndpoint: driver.Stack.InternalEndpoints,
	}

	testBin.Plan.Spec = buildAndAttachDataLayer{testBin.Plan.SourceLabel, testBin.Plan.Spec, makeRequestDataLayer(testReq)}

	// We build multi-platform binaries because we don't know if the target cluster
	// is actually multi-platform as well (although we could probably resolve it at
	// test setup time, i.e. now).
	bin, err := multiplatform.PrepareMultiPlatformImage(ctx, driver.SealedContext, testBin.Plan)
	if err != nil {
		return deploy.PreparedDeployable{}, err
	}

	testBinTag := registry.AllocateName(driver.Location.String())

	driverImage, err := compute.GetValue(ctx, oci.PublishResolvable(testBinTag, bin, testBin.Plan))
	if err != nil {
		return deploy.PreparedDeployable{}, err
	}

	container := runtime.ContainerRunOpts{
		Image:              driverImage,
		Command:            testBin.Command,
		Args:               testBin.Args,
		Env:                testBin.Env,
		WorkingDir:         testBin.WorkingDir,
		ReadOnlyFilesystem: false,
	}

	if driver.Debug {
		container.Args = append(container.Args, "--debug")
	}

	pkgId := naming.StableIDN(driver.TestRef.PackageName, 8)

	return deploy.PreparedDeployable{
		Ref:       driver.TestRef,
		SealedCtx: driver.SealedContext,
		Template: runtime.DeployableSpec{
			ErrorLocation:          driver.TestRef.AsPackageName(),
			PackageRef:             driver.TestRef,
			Description:            "Test Driver",
			Class:                  schema.DeployableClass_ONESHOT,
			Id:                     ids.NewRandomBase32ID(8),
			Name:                   fmt.Sprintf("%s-%s", driver.TestRef.Name, pkgId),
			MainContainer:          container,
			RuntimeConfig:          runtimeConfig,
			MountRuntimeConfigPath: constants.NamespaceConfigMount,
		},
		Resources: driver.Resources,
	}, nil
}
