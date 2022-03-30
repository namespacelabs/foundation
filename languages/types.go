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
)

type Integration interface {
	workspace.FrameworkHandler

	// Called on `fn build`, `fn deploy`.
	PrepareBuild(context.Context, Endpoints, provision.Server, bool /*isFocus*/) (build.Spec, error)
	PrepareRun(context.Context, provision.Server, *runtime.ServerRunOpts) error

	// Called on `fn tidy`
	TidyNode(context.Context, workspace.Location, *schema.Node) error
	TidyServer(context.Context, workspace.Location, *schema.Server) error

	// Called on `fn generate`.
	GenerateNode(*workspace.Package, []*schema.Node) ([]*schema.Definition, error)
	GenerateServer(*workspace.Package, []*schema.Node) ([]*schema.Definition, error)

	// Called on `fn dev`.
	PrepareDev(context.Context, provision.Server) (context.Context, DevObserver, error)
}

type DevObserver interface {
	io.Closer
	OnDeployment()
}

type Endpoints interface {
	GetEndpoints() []*schema.Endpoint
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

func (MaybePrepare) PrepareBuild(context.Context, Endpoints, provision.Server, bool) (build.Spec, error) {
	return nil, nil
}
func (MaybePrepare) PrepareRun(context.Context, provision.Server, *runtime.ServerRunOpts) error {
	return nil
}

type MaybeGenerate struct{}

func (MaybeGenerate) GenerateNode(*workspace.Package, []*schema.Node) ([]*schema.Definition, error) {
	return nil, nil
}
func (MaybeGenerate) GenerateServer(*workspace.Package, []*schema.Node) ([]*schema.Definition, error) {
	return nil, nil
}

type MaybeTidy struct{}

func (MaybeTidy) TidyNode(ctx context.Context, loc workspace.Location, server *schema.Node) error {
	return nil
}

func (MaybeTidy) TidyServer(ctx context.Context, loc workspace.Location, server *schema.Server) error {
	return nil
}

type NoDev struct{}

func (NoDev) PrepareDev(ctx context.Context, _ provision.Server) (context.Context, DevObserver, error) {
	return ctx, nil, nil
}
