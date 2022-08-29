// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cuefrontendopaque

import (
	"context"

	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/frontend"
	"namespacelabs.dev/foundation/schema"
)

type phase1plan struct {
	startupPlan *schema.StartupPlan
}

func (p1 phase1plan) EvalProvision(ctx context.Context, env ops.Environment, inputs frontend.ProvisionInputs) (frontend.ProvisionPlan, error) {
	var pdata frontend.ProvisionPlan

	pdata.Startup = phase2plan(p1)

	return pdata, nil
}
