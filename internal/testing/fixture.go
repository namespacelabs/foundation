// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package testing

import (
	"context"

	"namespacelabs.dev/foundation/build"
	"namespacelabs.dev/foundation/build/binary"
	"namespacelabs.dev/foundation/build/multiplatform"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/artifacts/registry"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/stack"
	"namespacelabs.dev/foundation/internal/testing/testboot"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/provision/deploy"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/compute"
)

const startupTestBinary = "namespacelabs.dev/foundation/std/startup/testdriver"

type TestOpts struct {
	Debug bool
}

type LoadSUTFunc func(context.Context, *workspace.PackageLoader, *schema.Test) ([]provision.Server, *stack.Stack, error)

func PrepareTest(ctx context.Context, pl *workspace.PackageLoader, env provision.Env, pkgname schema.PackageName, opts TestOpts, loadSUT LoadSUTFunc) (compute.Computable[oci.ImageID], error) {
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

	testBin, err := binary.Plan(ctx, testBinary, binary.BuildImageOpts{
		Platforms: runtime.For(env).HostPlatforms(),
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

	results := &testRun{
		TestName:       testDef.Name,
		Env:            env.BindWith(pl.Seal()),
		Plan:           deployPlan,
		Focus:          focusServers,
		Stack:          stack.Proto(),
		TestBinPkg:     testBinary.PackageName(),
		TestBinCommand: testBin.Command,
		TestBinImageID: fixtureImage,
		Debug:          opts.Debug,
	}

	return oci.PublishImage(tag, oci.MakeImage(oci.Scratch(), oci.MakeLayer("results", results))), nil
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
