// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cuefrontendopaque

import (
	"context"

	"namespacelabs.dev/foundation/internal/frontend"
	"namespacelabs.dev/foundation/internal/planning"
	"namespacelabs.dev/foundation/schema"
)

type phase2plan struct {
	startupPlan *schema.StartupPlan
}

func (s phase2plan) EvalStartup(ctx context.Context, env planning.Context, info frontend.StartupInputs, allocs []frontend.ValueWithPath) (*schema.StartupPlan, error) {
	return s.startupPlan, nil
}
