// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package opaque

import (
	"context"

	"namespacelabs.dev/foundation/build"
	"namespacelabs.dev/foundation/build/binary"
	"namespacelabs.dev/foundation/languages"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/workspace"
)

func Register() {
	languages.Register(schema.Framework_OPAQUE, impl{})
}

type impl struct {
	languages.MaybeGenerate
	languages.MaybeTidy
	languages.NoDev
}

func (impl) PrepareBuild(ctx context.Context, _ languages.AvailableBuildAssets, server provision.Server, isFocus bool) (build.Spec, error) {
	binRef := server.Proto().GetMainContainer().GetBinaryRef()

	pkg, err := server.SealedContext().LoadByName(ctx, binRef.AsPackageName())
	if err != nil {
		return nil, err
	}

	prep, err := binary.Plan(ctx, pkg, binRef.GetName(), server.SealedContext(), binary.BuildImageOpts{UsePrebuilts: true})
	if err != nil {
		return nil, err
	}

	return prep.Plan.Spec, nil
}

func (impl) PrepareRun(ctx context.Context, server provision.Server, run *runtime.ContainerRunOpts) error {
	binRef := server.Proto().GetMainContainer().GetBinaryRef()
	if binRef != nil {
		pkg, err := server.SealedContext().LoadByName(ctx, binRef.AsPackageName())
		if err != nil {
			return err
		}

		binary, err := binary.GetBinary(pkg, binRef.GetName())
		if err != nil {
			return err
		}

		config := binary.Config
		if config != nil {
			run.Command = config.Command
			run.Args = config.Args
			run.Env = config.Env
		}
	}

	return nil
}

func (impl) PreParseServer(ctx context.Context, loc pkggraph.Location, ext *workspace.ServerFrameworkExt) error {
	return nil
}

func (impl) PostParseServer(ctx context.Context, _ *workspace.Sealed) error {
	return nil
}

func (impl) DevelopmentPackages() []schema.PackageName {
	return nil
}
