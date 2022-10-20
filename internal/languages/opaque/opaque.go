// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package opaque

import (
	"context"

	"namespacelabs.dev/foundation/internal/build"
	"namespacelabs.dev/foundation/internal/build/assets"
	"namespacelabs.dev/foundation/internal/build/binary"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/languages"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/internal/planning"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

func Register() {
	languages.Register(schema.Framework_OPAQUE, OpaqueIntegration{})
}

type OpaqueIntegration struct {
	languages.MaybeGenerate
	languages.MaybeTidy // TODO implement tidy per parser.
	languages.NoDev
}

func (OpaqueIntegration) PrepareBuild(ctx context.Context, assets assets.AvailableBuildAssets, server planning.Server, isFocus bool) (build.Spec, error) {
	binRef := server.Proto().GetMainContainer().GetBinaryRef()

	if binRef == nil {
		return nil, fnerrors.InternalError("server binary is not set at %s", server.Location)
	}

	pkg, err := server.SealedContext().LoadByName(ctx, binRef.AsPackageName())
	if err != nil {
		return nil, err
	}

	prep, err := binary.Plan(ctx, pkg, binRef.GetName(), server.SealedContext(), assets,
		binary.BuildImageOpts{UsePrebuilts: true, IsFocus: isFocus})
	if err != nil {
		return nil, err
	}

	return prep.Plan.Spec, nil
}

func (OpaqueIntegration) PrepareRun(ctx context.Context, server planning.Server, run *runtime.ContainerRunOpts) error {
	binRef := server.Proto().GetMainContainer().GetBinaryRef()
	if binRef != nil {
		_, binary, err := pkggraph.LoadBinary(ctx, server.SealedContext(), binRef)
		if err != nil {
			return err
		}

		config := binary.Config
		if config != nil {
			run.WorkingDir = config.WorkingDir
			run.Command = config.Command
			run.Args = config.Args
			run.Env = config.Env
		}
	}

	return nil
}

func (OpaqueIntegration) PreParseServer(ctx context.Context, loc pkggraph.Location, ext *parsing.ServerFrameworkExt) error {
	return nil
}

func (OpaqueIntegration) PostParseServer(ctx context.Context, _ *parsing.Sealed) error {
	return nil
}

func (OpaqueIntegration) DevelopmentPackages() []schema.PackageName {
	return nil
}
