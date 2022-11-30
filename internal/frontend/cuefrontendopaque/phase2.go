// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cuefrontendopaque

import (
	"context"

	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

type phase2plan struct {
}

func (s phase2plan) EvalStartup(ctx context.Context, env pkggraph.Context, info pkggraph.StartupInputs, allocs []pkggraph.ValueWithPath) (*schema.StartupPlan, error) {
	return nil, nil
}
