// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package testing

import (
	"context"
	"fmt"

	"google.golang.org/protobuf/types/known/timestamppb"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/build"
	"namespacelabs.dev/foundation/internal/build/assets"
	"namespacelabs.dev/foundation/internal/build/binary"
	"namespacelabs.dev/foundation/internal/build/multiplatform"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/internal/planning"
	"namespacelabs.dev/foundation/internal/planning/deploy"
	"namespacelabs.dev/foundation/internal/planning/eval"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/internal/support/naming"
	"namespacelabs.dev/foundation/internal/testing/testboot"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/schema/storage"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/std/runtime/constants"
	"namespacelabs.dev/foundation/std/tasks"
	"namespacelabs.dev/go-ids"
)

type StoredTestResults struct {
	Bundle            *storage.TestResultBundle
	TestBundleSummary *storage.TestBundle
	TestPackage       schema.PackageName
	Packages          pkggraph.SealedPackageLoader
}

type TestOpts struct {
	ParentRunID    string
	Debug          bool
	OutputProgress bool
	KeepRuntime    bool // If true, don't release test-specific runtime resources (e.g. Kubernetes namespace).
}

func PrepareTest(ctx context.Context, pl *parsing.PackageLoader, env cfg.Context, testRef *schema.PackageRef, opts TestOpts) (compute.Computable[StoredTestResults], error) {
	testPkg, err := pl.LoadByName(ctx, testRef.AsPackageName())
	if err != nil {
		return nil, err
	}

	testDef, err := findTest(testPkg.Location, testPkg.Tests, testRef.Name)
	if err != nil {
		return nil, err
	}

	driverLoc, err := pl.Resolve(ctx, schema.PackageName(testDef.Driver.PackageName))
	if err != nil {
		return nil, err
	}

	deferred, err := runtime.ClassFor(ctx, env)
	if err != nil {
		return nil, fnerrors.AttachLocation(testPkg.Location, err)
	}

	purpose := fmt.Sprintf("Test cluster for %s", testPkg.Location)

	// This can block for a non-trivial amount of time.
	cluster, err := deferred.EnsureCluster(ctx, env.Configuration(), purpose)
	if err != nil {
		return nil, fnerrors.AttachLocation(testPkg.Location, err)
	}

	planner, err := cluster.Planner(ctx, env)
	if err != nil {
		return nil, err
	}

	platforms, err := planner.TargetPlatforms(ctx)
	if err != nil {
		return nil, err
	}

	stack, err := loadSUT(ctx, env, pl, testDef)
	if err != nil {
		return nil, fnerrors.NewWithLocation(testPkg.Location, "failed to load fixture: %w", err)
	}

	pack := &schema.ResourcePack{}
	if err := parsing.AddServersAsResources(ctx, pl, testRef, stack.Focus.PackageNames(), pack); err != nil {
		return nil, err
	}

	resources, err := parsing.LoadResources(ctx, pl, driverLoc, pack)
	if err != nil {
		return nil, err
	}

	// Must be no "pl" usage after this point:
	// All packages have been bound to the environment, and sealed.
	packages := pl.Seal()

	testBin, err := binary.PlanBinary(ctx, packages, env, driverLoc, testDef.Driver, assets.AvailableBuildAssets{}, binary.BuildImageOpts{
		Platforms: platforms,
	})
	if err != nil {
		return nil, err
	}

	registry := planner.Registry()

	sutServers := stack.Focus.PackageNamesAsString()
	runtimeConfig, err := deploy.TestStackToRuntimeConfig(stack, sutServers)
	if err != nil {
		return nil, err
	}

	testReq := &testboot.TestRequest{
		Endpoint:         stack.Endpoints,
		InternalEndpoint: stack.InternalEndpoints,
	}

	testBin.Plan.Spec = buildAndAttachDataLayer{testBin.Plan.SourceLabel, testBin.Plan.Spec, makeRequestDataLayer(testReq)}

	sealedCtx := pkggraph.MakeSealedContext(env, packages)
	// We build multi-platform binaries because we don't know if the target cluster
	// is actually multi-platform as well (although we could probably resolve it at
	// test setup time, i.e. now).
	bin, err := multiplatform.PrepareMultiPlatformImage(ctx, sealedCtx, testBin.Plan)
	if err != nil {
		return nil, err
	}

	testBinTag := registry.AllocateName(driverLoc.PackageName.String())

	driverImage, err := compute.GetValue(ctx, oci.PublishResolvable(testBinTag, bin, testBin.Plan))
	if err != nil {
		return nil, err
	}

	container := runtime.ContainerRunOpts{
		Image:              driverImage,
		Command:            testBin.Command,
		Args:               testBin.Args,
		Env:                testBin.Env,
		WorkingDir:         testBin.WorkingDir,
		ReadOnlyFilesystem: true,
	}

	if opts.Debug {
		container.Args = append(container.Args, "--debug")
	}

	pkgId := naming.StableIDN(testRef.PackageName, 8)

	testDriver := deploy.PreparedDeployable{
		Ref:       testRef,
		SealedCtx: sealedCtx,
		Template: runtime.DeployableSpec{
			ErrorLocation:          testRef.AsPackageName(),
			PackageRef:             testRef,
			Description:            "Test Driver",
			Class:                  schema.DeployableClass_ONESHOT,
			Id:                     ids.NewRandomBase32ID(8),
			Name:                   fmt.Sprintf("%s-%s", testRef.Name, pkgId),
			MainContainer:          container,
			RuntimeConfig:          runtimeConfig,
			MountRuntimeConfigPath: constants.NamespaceConfigMount,
		},
		Resources: resources,
	}

	deployPlan, err := deploy.PrepareDeployStack(ctx, env, planner, stack, testDriver)
	if err != nil {
		return nil, fnerrors.NewWithLocation(testPkg.Location, "failed to load stack: %w", err)
	}

	var results compute.Computable[*storage.TestResultBundle] = &testRun{
		SealedContext:    sealedCtx,
		Cluster:          cluster,
		TestRef:          testRef,
		Plan:             deployPlan,
		ServersUnderTest: sutServers,
		Stack:            stack.Proto(),
		Driver:           testDriver.Template,
		OutputProgress:   opts.OutputProgress,
	}

	createdTs := timestamppb.Now()

	testBundle := compute.Map(tasks.Action("test.to-bundle"),
		compute.Inputs().
			Computable("results", results).
			JSON("opts", opts).
			Proto("testDef", testDef).
			Strs("sut", sutServers).
			Proto("createdTs", createdTs),
		compute.Output{NotCacheable: true},
		func(ctx context.Context, deps compute.Resolved) (*storage.TestBundle, error) {
			bundle := compute.MustGetDepValue(deps, results, "results")
			return &storage.TestBundle{
				ParentRunId:      opts.ParentRunID,
				TestPackage:      testDef.PackageName,
				TestName:         testDef.Name,
				Result:           bundle.Result,
				ServersUnderTest: sutServers,
				Created:          createdTs,
				Completed:        timestamppb.Now(),
				EnvDiagnostics:   bundle.EnvDiagnostics,
			}, nil
		})

	return compute.Map(tasks.Action("test.make-results"),
		compute.Inputs().
			Indigestible("packages", packages).
			Computable("bundle", results).
			Computable("testBundle", testBundle),
		compute.Output{NotCacheable: true},
		func(ctx context.Context, deps compute.Resolved) (StoredTestResults, error) {
			return StoredTestResults{
				Bundle:            compute.MustGetDepValue(deps, results, "bundle"),
				TestBundleSummary: compute.MustGetDepValue(deps, testBundle, "testBundle"),
				TestPackage:       testRef.AsPackageName(),
				Packages:          packages,
			}, nil
		}), nil
}

func loadSUT(ctx context.Context, env cfg.Context, pl *parsing.PackageLoader, test *schema.Test) (*planning.Stack, error) {
	var suts []planning.Server

	for _, pkg := range test.ServersUnderTest {
		sut, err := planning.RequireServerWith(ctx, env, pl, schema.PackageName(pkg))
		if err != nil {
			return nil, err
		}
		suts = append(suts, sut)
	}

	stack, err := planning.ComputeStack(ctx, suts, planning.ProvisionOpts{PortRange: eval.DefaultPortRange()})
	if err != nil {
		return nil, err
	}

	return stack, nil
}

func findTest(loc pkggraph.Location, tests []*schema.Test, name string) (*schema.Test, error) {
	for _, t := range tests {
		if t.Name == name {
			return t, nil
		}
	}
	return nil, fnerrors.NewWithLocation(loc, "%s: test not found", name)
}

type buildAndAttachDataLayer struct {
	baseName  string
	spec      build.Spec
	dataLayer oci.NamedLayer
}

func (b buildAndAttachDataLayer) BuildImage(ctx context.Context, env pkggraph.SealedContext, conf build.Configuration) (compute.Computable[oci.Image], error) {
	base, err := b.spec.BuildImage(ctx, env, conf)
	if err != nil {
		return nil, err
	}
	return oci.MakeImage(b.baseName, oci.MakeNamedImage(b.baseName, base), b.dataLayer).Image(), nil
}

func (b buildAndAttachDataLayer) PlatformIndependent() bool {
	return b.spec.PlatformIndependent()
}
