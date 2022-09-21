// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package testing

import (
	"context"
	"fmt"
	"io/fs"

	"google.golang.org/protobuf/types/known/timestamppb"
	"namespacelabs.dev/foundation/build"
	"namespacelabs.dev/foundation/build/binary"
	"namespacelabs.dev/foundation/build/multiplatform"
	"namespacelabs.dev/foundation/engine/compute"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/artifacts/registry"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs/memfs"
	"namespacelabs.dev/foundation/internal/protos"
	"namespacelabs.dev/foundation/internal/stack"
	"namespacelabs.dev/foundation/internal/testing/testboot"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/provision/config"
	"namespacelabs.dev/foundation/provision/deploy"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/schema/storage"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/source/protos/resolver"
	"namespacelabs.dev/foundation/workspace/tasks"
)

type StoredTestResults struct {
	Bundle            *storage.TestResultBundle
	TestBundleSummary *storage.TestBundle
	ImageRef          oci.ImageID
	Package           schema.PackageName
}

type TestOpts struct {
	ParentRunID    string
	Debug          bool
	OutputProgress bool
	KeepRuntime    bool // If true, don't release test-specific runtime resources (e.g. Kubernetes namespace).
}

type LoadSUTFunc func(context.Context, *workspace.PackageLoader, *schema.Test) ([]provision.Server, *stack.Stack, error)

func PrepareTest(ctx context.Context, pl *workspace.PackageLoader, env pkggraph.SealedContext, testRef *schema.PackageRef, opts TestOpts, loadSUT LoadSUTFunc) (compute.Computable[StoredTestResults], error) {
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
		return nil, fnerrors.Wrap(testPkg.Location, err)
	}

	// This can block for a non-trivial amount of time.
	cluster, err := deferred.EnsureCluster(ctx, env.Configuration())
	if err != nil {
		return nil, fnerrors.Wrap(testPkg.Location, err)
	}

	planner := cluster.Planner(env)

	platforms, err := planner.TargetPlatforms(ctx)
	if err != nil {
		return nil, err
	}

	testBin, err := binary.PlanBinary(ctx, driverLoc, testDef.Driver, env, binary.BuildImageOpts{
		Platforms: platforms,
	})
	if err != nil {
		return nil, err
	}

	testBinTag, err := registry.AllocateName(ctx, env, driverLoc.PackageName)
	if err != nil {
		return nil, err
	}

	sut, stack, err := loadSUT(ctx, pl, testDef)
	if err != nil {
		return nil, fnerrors.UserError(testPkg.Location, "failed to load fixture: %w", err)
	}

	deployPlan, err := deploy.PrepareDeployStack(ctx, env, planner, stack, sut)
	if err != nil {
		return nil, fnerrors.UserError(testPkg.Location, "failed to load stack: %w", err)
	}

	testReq := &testboot.TestRequest{
		Endpoint:         stack.Endpoints,
		InternalEndpoint: stack.InternalEndpoints,
	}

	testBin.Plan.Spec = buildAndAttachDataLayer{testBin.Plan.SourceLabel, testBin.Plan.Spec, makeRequestDataLayer(testReq)}

	// We build multi-platform binaries because we don't know if the target cluster
	// is actually multi-platform as well (although we could probably resolve it at
	// test setup time, i.e. now).
	bin, err := multiplatform.PrepareMultiPlatformImage(ctx, env, testBin.Plan)
	if err != nil {
		return nil, err
	}

	fixtureImage := oci.PublishResolvable(testBinTag, bin)

	var sutServers []string
	for _, srv := range sut {
		sutServers = append(sutServers, srv.Proto().GetPackageName())
	}

	tag, err := registry.AllocateName(ctx, env, testPkg.PackageName())
	if err != nil {
		return nil, err
	}

	packages := pl.Seal()

	runtimeConfig, err := deploy.TestStackToRuntimeConfig(stack, sutServers)
	if err != nil {
		return nil, err
	}

	var results compute.Computable[*storage.TestResultBundle] = &testRun{
		SealedContext:    pkggraph.MakeSealedContext(env, packages),
		Cluster:          cluster,
		TestRef:          testRef,
		Plan:             deployPlan,
		ServersUnderTest: sutServers,
		Stack:            stack.Proto(),
		RuntimeConfig:    runtimeConfig,
		TestBinCommand:   testBin.Command,
		TestBinImageID:   fixtureImage,
		Debug:            opts.Debug,
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

	toFS := compute.Map(tasks.Action("test.to-fs"),
		compute.Inputs().Computable("bundle", results).
			Computable("testBundle", testBundle).
			Indigestible("packages", packages).
			JSON("opts", opts).
			Proto("test", testDef).
			Proto("createdTs", createdTs).
			Strs("sut", sutServers),
		compute.Output{NotCacheable: true},
		func(ctx context.Context, deps compute.Resolved) (fs.FS, error) {
			bundle := compute.MustGetDepValue(deps, results, "bundle")
			tostore := protos.Clone(compute.MustGetDepValue(deps, testBundle, "testBundle"))

			// We only add timestamps in the transformation step, as it would
			// otherwise break the ability to cache test results.

			var fsys memfs.FS

			if bundle.TestLog != nil {
				tostore.TestLog = &storage.LogRef{
					PackageName:   bundle.TestLog.PackageName,
					ContainerName: bundle.TestLog.ContainerName,
					LogFile:       "test.log",
					LogSize:       uint64(len(bundle.TestLog.Output)),
				}
				fsys.Add("test.log", bundle.TestLog.Output)
			}

			for _, s := range bundle.ServerLog {
				x, err := schema.DigestOf(s.PackageName, s.ContainerName)
				if err != nil {
					return nil, err
				}

				logFile := fmt.Sprintf("server/%s.log", x.Hex)
				fsys.Add(logFile, s.Output)

				tostore.ServerLog = append(tostore.ServerLog, &storage.LogRef{
					PackageName:   s.PackageName,
					ContainerName: s.ContainerName,
					ContainerKind: s.ContainerKind,
					LogFile:       logFile,
					LogSize:       uint64(len(s.Output)),
				})
			}

			messages, err := protos.SerializeOpts{JSON: true, Resolver: resolver.NewResolver(ctx, packages)}.Serialize(tostore)
			if err != nil {
				return nil, fnerrors.InternalError("failed to marshal results: %w", err)
			}

			fsys.Add("testbundle.json", messages[0].JSON)
			fsys.Add("testbundle.binarypb", messages[0].Binary)

			// Produce an image that can be rehydrated.
			if err := (config.DehydrateOpts{}).DehydrateTo(ctx, &fsys, bundle.DeployPlan.Environment, bundle.DeployPlan.Stack,
				bundle.DeployPlan.IngressFragment, bundle.ComputedConfigurations); err != nil {
				return nil, err
			}

			return &fsys, nil
		})

	imageID := oci.PublishImage(tag, oci.MakeImageFromScratch("test-results", oci.MakeLayer("test-results", toFS))).ImageID()

	return compute.Map(tasks.Action("test.make-results"),
		compute.Inputs().
			Computable("stored", imageID).
			Computable("bundle", results).
			Computable("testBundle", testBundle),
		compute.Output{NotCacheable: true},
		func(ctx context.Context, deps compute.Resolved) (StoredTestResults, error) {
			return StoredTestResults{
				Bundle:            compute.MustGetDepValue(deps, results, "bundle"),
				TestBundleSummary: compute.MustGetDepValue(deps, testBundle, "testBundle"),
				ImageRef:          compute.MustGetDepValue(deps, imageID, "stored"),
				Package:           testRef.AsPackageName(),
			}, nil
		}), nil
}

func findTest(loc pkggraph.Location, tests []*schema.Test, name string) (*schema.Test, error) {
	for _, t := range tests {
		if t.Name == name {
			return t, nil
		}
	}
	return nil, fnerrors.UserError(loc, "%s: test not found", name)
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
