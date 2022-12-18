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
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/internal/planning"
	"namespacelabs.dev/foundation/internal/planning/deploy"
	"namespacelabs.dev/foundation/internal/planning/eval"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/schema/storage"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/std/tasks"
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

	// This can block for some time.
	planner, err := deferred.Planner(ctx, env, purpose)
	if err != nil {
		return nil, fnerrors.AttachLocation(testPkg.Location, err)
	}

	stack, err := loadSUT(ctx, env, pl, testDef)
	if err != nil {
		return nil, fnerrors.NewWithLocation(testPkg.Location, "failed to load fixture: %w", err)
	}

	pack := &schema.ResourcePack{}
	if err := parsing.AddServersAsResources(ctx, pl, testRef, stack.Focus.PackageNames(), pack); err != nil {
		return nil, err
	}

	resources, err := parsing.LoadResources(ctx, pl, testPkg, testRef.Canonical(), pack)
	if err != nil {
		return nil, err
	}

	// Must be no "pl" usage after this point:
	// All packages have been bound to the environment, and sealed.
	packages := pl.Seal()

	sealedCtx := pkggraph.MakeSealedContext(env, packages)

	driver := &testDriver{
		TestRef:       testRef,
		Location:      driverLoc,
		Definition:    testDef.Driver,
		Stack:         stack,
		SealedContext: sealedCtx,
		Planner:       planner,
		Resources:     resources,
		Debug:         opts.Debug,
	}

	deployPlan, err := deploy.PrepareDeployStack(ctx, env, planner, planner.Registry(), stack, driver)
	if err != nil {
		return nil, fnerrors.NewWithLocation(testPkg.Location, "failed to load stack: %w", err)
	}

	sutServers := stack.Focus

	var results compute.Computable[*storage.TestResultBundle] = &testRun{
		SealedContext:    sealedCtx,
		Planner:          planner,
		TestRef:          testRef,
		Plan:             deployPlan,
		ServersUnderTest: sutServers,
		Stack:            stack.Proto(),
		Driver:           driver,
		OutputProgress:   opts.OutputProgress,
	}

	createdTs := timestamppb.Now()

	testBundle := compute.Transform("to-bundle", results, func(ctx context.Context, bundle *storage.TestResultBundle) (*storage.TestBundle, error) {
		return &storage.TestBundle{
			ParentRunId:      opts.ParentRunID,
			TestPackage:      testDef.PackageName,
			TestName:         testDef.Name,
			Result:           bundle.Result,
			ServersUnderTest: sutServers.PackageNamesAsString(),
			Created:          createdTs,
			Started:          bundle.Started,
			Completed:        bundle.Completed,
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
