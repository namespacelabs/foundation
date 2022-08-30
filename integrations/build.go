// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package integrations

import (
	"context"

	"namespacelabs.dev/foundation/build"
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
