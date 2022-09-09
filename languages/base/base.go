// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package base

import (
	"context"

	"namespacelabs.dev/foundation/build"
	"namespacelabs.dev/foundation/integrations/shared"
	"namespacelabs.dev/foundation/languages"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/workspace"
)

func Register() {
	languages.Register(schema.Framework_BASE, impl{})
}

type impl struct {
	languages.MaybeGenerate
	languages.MaybeTidy
	languages.NoDev
}

func (impl) PrepareBuild(ctx context.Context, _ languages.AvailableBuildAssets, server provision.Server, isFocus bool) (build.Spec, error) {
	return shared.PrepareBuild(ctx, server.Location, server.Integration(), isFocus)
}

func (impl) PrepareRun(ctx context.Context, server provision.Server, run *runtime.ServerRunOpts) error {
	return shared.PrepareRun(ctx, server, run)
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
