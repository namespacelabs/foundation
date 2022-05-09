// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package testing

import (
	"context"

	"namespacelabs.dev/foundation/build/binary"
	"namespacelabs.dev/foundation/build/multiplatform"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/artifacts/registry"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/compute"
)

const controllerPkg = "namespacelabs.dev/foundation/std/testing/controller"

func EnsureController(ctx context.Context, pl *workspace.PackageLoader, fac factory) error {
	env := fac.PrepareControllerEnv()

	pkg, err := pl.LoadByName(ctx, controllerPkg)
	if err != nil {
		return err
	}

	platforms, err := runtime.For(ctx, env).TargetPlatforms(ctx)
	if err != nil {
		return err
	}

	prepared, err := binary.Plan(ctx, pkg, binary.BuildImageOpts{
		Platforms: platforms,
	})
	if err != nil {
		return err
	}

	bid := provision.NewBuildID()
	binTag, err := registry.AllocateName(ctx, env, controllerPkg, bid)
	if err != nil {
		return err
	}

	bin, err := multiplatform.PrepareMultiPlatformImage(ctx, env, prepared.Plan)
	if err != nil {
		return err
	}

	fixtureImage := oci.PublishResolvable(binTag, bin)

	// TODO model async when we run tests in parallel #344
	img, err := compute.Get(ctx, fixtureImage)
	if err != nil {
		return err
	}

	runOpts := runtime.ServerRunOpts{
		Image:              img.Value,
		Command:            prepared.Command,
		Args:               nil,
		ReadOnlyFilesystem: true,
	}

	return runtime.For(ctx, env).RunController(ctx, runOpts)
}
