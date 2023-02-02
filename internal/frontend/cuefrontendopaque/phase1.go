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
	StartupPlan    *schema.StartupPlan
	DeclaredStack  []schema.PackageName
	Sidecars       []*schema.Container
	InitContainers []*schema.Container
	Naming         *schema.Naming
}

func (p1 phase1plan) EvalProvision(ctx context.Context, env cfg.Context, inputs pkggraph.ProvisionInputs) (pkggraph.ProvisionPlan, error) {
	var pdata pkggraph.ProvisionPlan

	pdata.StartupPlan = p1.StartupPlan
	pdata.DeclaredStack = p1.DeclaredStack
	pdata.Sidecars = p1.Sidecars
	pdata.Inits = p1.InitContainers
	pdata.Naming = p1.Naming

	return pdata, nil
}
