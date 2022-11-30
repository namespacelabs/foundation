// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cuefrontendopaque

import (
	"context"

	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/pkggraph"
)

type phase1plan struct {
	sidecars       []*schema.SidecarContainer
	initContainers []*schema.SidecarContainer
	naming         *schema.Naming
}

func (p1 phase1plan) EvalProvision(ctx context.Context, env cfg.Context, inputs pkggraph.ProvisionInputs) (pkggraph.ProvisionPlan, error) {
	var pdata pkggraph.ProvisionPlan

	pdata.Startup = phase2plan{}

	pdata.Sidecars = p1.sidecars
	pdata.Inits = p1.initContainers
	pdata.Naming = p1.naming

	return pdata, nil
}
