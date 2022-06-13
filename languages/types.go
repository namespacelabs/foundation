// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package languages

import (
	"context"
	"io"

	"namespacelabs.dev/foundation/build"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/compute"
)

type AvailableBuildAssets struct {
	IngressFragments compute.Computable[[]*schema.IngressFragment]
}

type Integration interface {
	workspace.FrameworkHandler

	// Called on `fn build`, `fn deploy`.
	PrepareBuild(context.Context, AvailableBuildAssets, provision.Server, bool /*isFocus*/) (build.Spec, error)
	PrepareRun(context.Context, provision.Server, *runtime.ServerRunOpts) error

	// Called on `fn tidy`
	TidyWorkspace(context.Context, []*workspace.Package) error
	TidyNode(context.Context, workspace.Packages, *workspace.Package) error
	TidyServer(context.Context, workspace.Packages, workspace.Location, *schema.Server) error

	// Called on `fn generate`.
	GenerateNode(*workspace.Package, []*schema.Node) ([]*schema.SerializedInvocation, error)
	GenerateServer(*workspace.Package, []*schema.Node) ([]*schema.SerializedInvocation, error)

	// Called on `fn dev`.
	PrepareDev(context.Context, provision.Server) (context.Context, DevObserver, error)
}

type DevObserver interface {
	io.Closer
	OnDeployment()
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

func (MaybePrepare) PrepareBuild(context.Context, AvailableBuildAssets, provision.Server, bool) (build.Spec, error) {
	return nil, nil
}
func (MaybePrepare) PrepareRun(context.Context, provision.Server, *runtime.ServerRunOpts) error {
	return nil
}

type MaybeGenerate struct{}

func (MaybeGenerate) GenerateNode(*workspace.Package, []*schema.Node) ([]*schema.SerializedInvocation, error) {
	return nil, nil
}
func (MaybeGenerate) GenerateServer(*workspace.Package, []*schema.Node) ([]*schema.SerializedInvocation, error) {
	return nil, nil
}

type MaybeTidy struct{}

func (MaybeTidy) TidyWorkspace(context.Context, []*workspace.Package) error {
	return nil
}

func (MaybeTidy) TidyNode(ctx context.Context, pkgs workspace.Packages, p *workspace.Package) error {
	return nil
}

func (MaybeTidy) TidyServer(ctx context.Context, pkgs workspace.Packages, loc workspace.Location, server *schema.Server) error {
	return nil
}

type NoDev struct{}

func (NoDev) PrepareDev(ctx context.Context, _ provision.Server) (context.Context, DevObserver, error) {
	return ctx, nil, nil
}
