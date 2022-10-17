// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cuefrontendopaque

import (
	"context"

	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/pkggraph"
)

type phase1plan struct {
	startupPlan    *schema.StartupPlan
	declaredStack  []schema.PackageName
	sidecars       []*schema.SidecarContainer
	initContainers []*schema.SidecarContainer
}

func (p1 phase1plan) EvalProvision(ctx context.Context, env cfg.Context, inputs pkggraph.ProvisionInputs) (pkggraph.ProvisionPlan, error) {
	var pdata pkggraph.ProvisionPlan

	pdata.Startup = phase2plan{startupPlan: p1.startupPlan}

	pdata.DeclaredStack = p1.declaredStack
	pdata.Sidecars = p1.sidecars
	pdata.Inits = p1.initContainers

	return pdata, nil
}
