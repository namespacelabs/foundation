// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package languages

import (
	"context"
	"io"

	"namespacelabs.dev/foundation/internal/build"
	"namespacelabs.dev/foundation/internal/build/assets"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/internal/planning"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/pkggraph"
)

type Integration interface {
	parsing.FrameworkHandler

	// Called on `ns build`, `ns deploy`.
	PrepareBuild(context.Context, assets.AvailableBuildAssets, planning.Server, bool /*isFocus*/) (build.Spec, error)
	PrepareRun(context.Context, planning.Server, *runtime.ContainerRunOpts) error

	// Called on `ns tidy`
	TidyWorkspace(context.Context, cfg.Context, []*pkggraph.Package) error
	TidyNode(context.Context, cfg.Context, pkggraph.PackageLoader, *pkggraph.Package) error
	TidyServer(context.Context, cfg.Context, pkggraph.PackageLoader, pkggraph.Location, *schema.Server) error

	// Called on `ns generate`.
	GenerateNode(*pkggraph.Package, []*schema.Node) ([]*schema.SerializedInvocation, error)
	GenerateServer(*pkggraph.Package, []*schema.Node) ([]*schema.SerializedInvocation, error)

	// Called on `ns dev`.
	PrepareDev(context.Context, runtime.ClusterNamespace, planning.Server) (context.Context, DevObserver, error)
}

type DevObserver interface {
	io.Closer
	OnDeployment(context.Context)
}

var (
	mapping = map[string]Integration{}
)

func Register(fmwk schema.Framework, i Integration) {
	mapping[fmwk.String()] = i
	parsing.RegisterFrameworkHandler(fmwk, i)
}

func IntegrationFor(fmwk schema.Framework) Integration {
	return mapping[fmwk.String()]
}

type MaybePrepare struct{}

func (MaybePrepare) PrepareBuild(context.Context, assets.AvailableBuildAssets, planning.Server, bool) (build.Spec, error) {
	return nil, nil
}
func (MaybePrepare) PrepareRun(context.Context, planning.Server, *runtime.ContainerRunOpts) error {
	return nil
}

type MaybeGenerate struct{}

func (MaybeGenerate) GenerateNode(*pkggraph.Package, []*schema.Node) ([]*schema.SerializedInvocation, error) {
	return nil, nil
}
func (MaybeGenerate) GenerateServer(*pkggraph.Package, []*schema.Node) ([]*schema.SerializedInvocation, error) {
	return nil, nil
}

type MaybeTidy struct{}

func (MaybeTidy) TidyWorkspace(context.Context, cfg.Context, []*pkggraph.Package) error {
	return nil
}

func (MaybeTidy) TidyNode(context.Context, cfg.Context, pkggraph.PackageLoader, *pkggraph.Package) error {
	return nil
}

func (MaybeTidy) TidyServer(context.Context, cfg.Context, pkggraph.PackageLoader, pkggraph.Location, *schema.Server) error {
	return nil
}

type NoDev struct{}

func (NoDev) PrepareDev(ctx context.Context, _ runtime.ClusterNamespace, _ planning.Server) (context.Context, DevObserver, error) {
	return ctx, nil, nil
}
