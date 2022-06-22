// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package testing

import (
	"context"
	"fmt"
	"io/fs"

	"namespacelabs.dev/foundation/build"
	"namespacelabs.dev/foundation/build/binary"
	"namespacelabs.dev/foundation/build/multiplatform"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/artifacts/registry"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs/memfs"
	"namespacelabs.dev/foundation/internal/protos"
	"namespacelabs.dev/foundation/internal/stack"
	"namespacelabs.dev/foundation/internal/testing/testboot"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/provision/deploy"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/runtime/kubernetes"
	"namespacelabs.dev/foundation/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/runtime/kubernetes/vcluster"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/devhost"
	"namespacelabs.dev/foundation/workspace/source/protos/resolver"
	"namespacelabs.dev/foundation/workspace/tasks"
)

const startupTestBinary = "namespacelabs.dev/foundation/std/startup/testdriver"

type StoredTestResults struct {
	Bundle   *PreStoredTestBundle
	ImageRef oci.ImageID
	Package  schema.PackageName
}

type TestOpts struct {
	Debug          bool
	OutputProgress bool
	KeepRuntime    bool // If true, don't release test-specific runtime resources (e.g. Kubernetes namespace).
}

type LoadSUTFunc func(context.Context, *workspace.PackageLoader, *schema.Test) ([]provision.Server, *stack.Stack, error)

func PrepareTest(ctx context.Context, pl *workspace.PackageLoader, env provision.Env, pkgname schema.PackageName, opts TestOpts, loadSUT LoadSUTFunc) (compute.Computable[StoredTestResults], error) {
	testPkg, err := pl.LoadByName(ctx, pkgname)
	if err != nil {
		return nil, err
	}

	var testDef *schema.Test
	var testBinary *workspace.Package
	var testResultsPrefix string

	bid := provision.NewBuildID()

	if testPkg.Server != nil {
		startupTest, err := pl.LoadByName(ctx, startupTestBinary)
		if err != nil {
			return nil, err
		}

		if startupTest.Binary == nil {
			return nil, fnerrors.InternalError("expected %q to be a binary", startupTestBinary)
		}

		testDef = &schema.Test{
			PackageName: testPkg.PackageName().String(),
			Name:        "startup-test",
			ServersUnderTest: []string{
				testPkg.Server.PackageName,
			},
		}

		testBinary = startupTest
		testResultsPrefix = "startup-test-"
	} else if testPkg.Test != nil {
		testDef = testPkg.Test
		testBinary = &workspace.Package{
			Binary:   testPkg.Test.Driver,
			Location: testPkg.Location,
		}

	} else {
		return nil, fnerrors.UserError(pkgname, "expected a test definition")
	}

	platforms, err := runtime.For(ctx, env).TargetPlatforms(ctx)
	if err != nil {
		return nil, err
	}

	testBin, err := binary.Plan(ctx, testBinary, binary.BuildImageOpts{
		Platforms: platforms,
	})
	if err != nil {
		return nil, err
	}

	testBinTag, err := registry.AllocateName(ctx, env, testBinary.PackageName(), bid)
	if err != nil {
		return nil, err
	}

	focus, stack, err := loadSUT(ctx, pl, testDef)
	if err != nil {
		return nil, fnerrors.UserError(testPkg.Location, "failed to load fixture: %w", err)
	}

	deployPlan, err := deploy.PrepareDeployStack(ctx, env, stack, focus)
	if err != nil {
		return nil, fnerrors.UserError(testPkg.Location, "failed to load stack: %w", err)
	}

	testReq := &testboot.TestRequest{
		Endpoint:         stack.Endpoints,
		InternalEndpoint: stack.InternalEndpoints,
	}

	testBin.Plan.Spec = buildAndAttachDataLayer{testBin.Plan.Spec, makeRequestDataLayer(testReq)}

	// We build multi-platform binaries because we don't know if the target cluster
	// is actually multi-platform as well (although we could probably resolve it at
	// test setup time, i.e. now).
	bin, err := multiplatform.PrepareMultiPlatformImage(ctx, env, testBin.Plan)
	if err != nil {
		return nil, err
	}

	fixtureImage := oci.PublishResolvable(testBinTag, bin)

	var focusServers []string
	for _, srv := range focus {
		focusServers = append(focusServers, srv.Proto().GetPackageName())
	}

	tag, err := registry.AllocateName(ctx, env, testPkg.PackageName(), bid.WithSuffix(testResultsPrefix+"results"))
	if err != nil {
		return nil, err
	}

	packages := pl.Seal()

	results := &testRun{
		TestName:       testDef.Name,
		Env:            env.BindWith(packages),
		Plan:           deployPlan,
		Focus:          focusServers,
		EnvProto:       env.Proto(),
		Workspace:      env.Workspace(),
		Stack:          stack.Proto(),
		TestBinPkg:     testBinary.PackageName(),
		TestBinCommand: testBin.Command,
		TestBinImageID: fixtureImage,
		Debug:          opts.Debug,
		OutputProgress: opts.OutputProgress,
		VCluster:       maybeCreateVCluster(env),
	}

	toFS := compute.Map(tasks.Action("test.to-fs"),
		compute.Inputs().Computable("bundle", results).Indigestible("packages", packages),
		compute.Output{},
		func(ctx context.Context, deps compute.Resolved) (fs.FS, error) {
			computed, ok := compute.GetDepWithType[*PreStoredTestBundle](deps, "bundle")
			if !ok {
				return nil, fnerrors.InternalError("results are missing")
			}

			bundle := computed.Value

			var fsys memfs.FS

			stored := &TestBundle{
				Result: bundle.Result,
				TestLog: &LogRef{
					PackageName:   bundle.TestLog.PackageName,
					ContainerName: bundle.TestLog.ContainerName,
					LogFile:       "test.log",
				},
			}

			fsys.Add("test.log", bundle.TestLog.Output)
			for _, s := range bundle.ServerLog {
				x, err := schema.DigestOf(s.PackageName, s.ContainerName)
				if err != nil {
					return nil, err
				}

				logFile := fmt.Sprintf("server/%s.log", x.Hex)
				fsys.Add(logFile, s.Output)

				stored.ServerLog = append(stored.ServerLog, &LogRef{
					PackageName:   s.PackageName,
					ContainerName: s.ContainerName,
					LogFile:       logFile,
				})
			}

			messages, err := protos.SerializeOpts{JSON: true, Resolver: resolver.NewResolver(ctx, packages)}.Serialize(stored)
			if err != nil {
				return nil, fnerrors.InternalError("failed to marshal results: %w", err)
			}

			fsys.Add("bundle.json", messages[0].JSON)
			fsys.Add("bundle.binarypb", messages[0].Binary)

			return &fsys, nil
		})

	imageID := oci.PublishImage(tag, oci.MakeImage(oci.Scratch(), oci.MakeLayer("results", toFS)))

	return compute.Map(tasks.Action("test.make-results"), compute.Inputs().Computable("stored", imageID).Computable("bundle", results), compute.Output{},
		func(ctx context.Context, deps compute.Resolved) (StoredTestResults, error) {
			return StoredTestResults{
				Bundle:   compute.MustGetDepValue[*PreStoredTestBundle](deps, results, "bundle"),
				ImageRef: compute.MustGetDepValue(deps, imageID, "stored"),
				Package:  pkgname,
			}, nil
		}), nil
}

type buildAndAttachDataLayer struct {
	spec      build.Spec
	dataLayer compute.Computable[oci.Layer]
}

func (b buildAndAttachDataLayer) BuildImage(ctx context.Context, env ops.Environment, conf build.Configuration) (compute.Computable[oci.Image], error) {
	base, err := b.spec.BuildImage(ctx, env, conf)
	if err != nil {
		return nil, err
	}
	return oci.MakeImage(base, b.dataLayer), nil
}

func (b buildAndAttachDataLayer) PlatformIndependent() bool {
	return b.spec.PlatformIndependent()
}

func maybeCreateVCluster(env provision.Env) compute.Computable[*vcluster.VCluster] {
	if !UseVClusters {
		return nil
	}

	hostConfig, err := client.ComputeHostConfig(env.DevHost(), devhost.ByEnvironment(env.Proto()))
	if err != nil {
		return compute.Error[*vcluster.VCluster](err)
	}

	ns := kubernetes.ModuleNamespace(env.Workspace(), env.Proto())

	return vcluster.Create(env.Proto(), hostConfig, ns)
}
