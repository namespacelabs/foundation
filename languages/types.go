// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package languages

import (
	"context"
	"io"

	"namespacelabs.dev/foundation/build"
	"namespacelabs.dev/foundation/engine/compute"
	"namespacelabs.dev/foundation/provision/parsed"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/std/planning"
	"namespacelabs.dev/foundation/workspace"
)

type AvailableBuildAssets struct {
	IngressFragments compute.Computable[[]*schema.IngressFragment]
}

type Integration interface {
	workspace.FrameworkHandler

	// Called on `ns build`, `ns deploy`.
	PrepareBuild(context.Context, AvailableBuildAssets, parsed.Server, bool /*isFocus*/) (build.Spec, error)
	PrepareRun(context.Context, parsed.Server, *runtime.ContainerRunOpts) error

	// Called on `ns tidy`
	TidyWorkspace(context.Context, planning.Context, []*pkggraph.Package) error
	TidyNode(context.Context, planning.Context, pkggraph.PackageLoader, *pkggraph.Package) error
	TidyServer(context.Context, planning.Context, pkggraph.PackageLoader, pkggraph.Location, *schema.Server) error

	// Called on `ns generate`.
	GenerateNode(*pkggraph.Package, []*schema.Node) ([]*schema.SerializedInvocation, error)
	GenerateServer(*pkggraph.Package, []*schema.Node) ([]*schema.SerializedInvocation, error)

	// Called on `ns dev`.
	PrepareDev(context.Context, runtime.ClusterNamespace, parsed.Server) (context.Context, DevObserver, error)
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
	workspace.RegisterFrameworkHandler(fmwk, i)
}

func IntegrationFor(fmwk schema.Framework) Integration {
	return mapping[fmwk.String()]
}

type MaybePrepare struct{}

func (MaybePrepare) PrepareBuild(context.Context, AvailableBuildAssets, parsed.Server, bool) (build.Spec, error) {
	return nil, nil
}
func (MaybePrepare) PrepareRun(context.Context, parsed.Server, *runtime.ContainerRunOpts) error {
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

func (MaybeTidy) TidyWorkspace(context.Context, planning.Context, []*pkggraph.Package) error {
	return nil
}

func (MaybeTidy) TidyNode(context.Context, planning.Context, pkggraph.PackageLoader, *pkggraph.Package) error {
	return nil
}

func (MaybeTidy) TidyServer(context.Context, planning.Context, pkggraph.PackageLoader, pkggraph.Location, *schema.Server) error {
	return nil
}

type NoDev struct{}

func (NoDev) PrepareDev(ctx context.Context, _ runtime.ClusterNamespace, _ parsed.Server) (context.Context, DevObserver, error) {
	return ctx, nil, nil
}
