// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package shared

import (
	"context"

	"namespacelabs.dev/foundation/build"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

type BuildIntegration interface {
	PrepareBuild(context.Context, pkggraph.Location, *schema.Integration, bool /*observeChanges*/) (build.Spec, error)
	PrepareRun(context.Context, provision.Server, *runtime.ServerRunOpts) error
}

var (
	buildIntegrations = map[string]BuildIntegration{}
)

func RegisterBuildIntegration(kind string, i BuildIntegration) {
	buildIntegrations[kind] = i
}

func BuildIntegrationFor(kind string) BuildIntegration {
	return buildIntegrations[kind]
}

func PrepareBuild(ctx context.Context, loc pkggraph.Location, integration *schema.Integration, observeChanges bool) (build.Spec, error) {
	bi, err := buildIntegrationFor(loc, integration)
	if err != nil {
		return nil, err
	}
	return bi.PrepareBuild(ctx, loc, integration, observeChanges)
}

func PrepareRun(ctx context.Context, server provision.Server, opts *runtime.ServerRunOpts) error {
	integration, err := buildIntegrationFor(server.Location, server.Integration())
	if err != nil {
		return err
	}
	return integration.PrepareRun(ctx, server, opts)
}

func buildIntegrationFor(loc pkggraph.Location, integration *schema.Integration) (BuildIntegration, error) {
	bi := BuildIntegrationFor(integration.Kind)
	if bi == nil {
		return nil, fnerrors.UserError(loc, "Unknown integration: %q", integration.Kind)
	}
	return bi, nil
}
