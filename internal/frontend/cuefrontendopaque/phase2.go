// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cuefrontendopaque

import (
	"context"

	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

type phase2plan struct {
	startupPlan *schema.StartupPlan
}

func (s phase2plan) EvalStartup(ctx context.Context, env pkggraph.Context, info pkggraph.StartupInputs, allocs []pkggraph.ValueWithPath) (*schema.StartupPlan, error) {
	return s.startupPlan, nil
}
