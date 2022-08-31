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
)

type BuildIntegration interface {
	PrepareBuild(context.Context, provision.Server, bool /*observeChanges*/) (build.Spec, error)
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

func PrepareBuild(ctx context.Context, server provision.Server, observeChanges bool) (build.Spec, error) {
	integration, err := buildIntegrationFor(server)
	if err != nil {
		return nil, err
	}
	return integration.PrepareBuild(ctx, server, observeChanges)
}

func PrepareRun(ctx context.Context, server provision.Server, opts *runtime.ServerRunOpts) error {
	integration, err := buildIntegrationFor(server)
	if err != nil {
		return err
	}
	return integration.PrepareRun(ctx, server, opts)
}

func buildIntegrationFor(server provision.Server) (BuildIntegration, error) {
	integration := BuildIntegrationFor(server.Integration().Kind)
	if integration == nil {
		return nil, fnerrors.UserError(server.Location, "Unknown integration: %q", server.Integration().Kind)
	}
	return integration, nil
}
