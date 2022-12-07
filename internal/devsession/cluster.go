// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package devsession

import (
	"context"
	"fmt"

	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/integrations"
	"namespacelabs.dev/foundation/internal/planning/deploy"
	"namespacelabs.dev/foundation/internal/portforward"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/orchestration"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
)

type updateCluster struct {
	env       cfg.Context
	cluster   runtime.ClusterNamespace
	observers []integrations.DevObserver

	plan  compute.Computable[*deploy.Plan]
	stack *schema.Stack
	focus []schema.PackageName

	pfw *portforward.PortForward
}

func newUpdateCluster(env cfg.Context, cluster runtime.ClusterNamespace, stack *schema.Stack, focus []schema.PackageName, observers []integrations.DevObserver, plan compute.Computable[*deploy.Plan], pfw *portforward.PortForward) *updateCluster {
	return &updateCluster{
		env:       env,
		cluster:   cluster,
		observers: observers,
		plan:      plan,
		stack:     stack,
		focus:     focus,
		pfw:       pfw,
	}
}

func (pi *updateCluster) Inputs() *compute.In {
	return compute.Inputs().Computable("plan", pi.plan).Proto("stack", pi.stack).JSON("focus", pi.focus)
}

func (pi *updateCluster) Updated(ctx context.Context, deps compute.Resolved) error {
	fmt.Fprintf(console.Debug(ctx), "devworkflow: updatedCluster.Updated\n")

	plan := compute.MustGetDepValue(deps, pi.plan, "plan")

	var focus schema.PackageList
	focus.AddMultiple(pi.focus...)

	deployPlan := deploy.Serialize(pi.env.Workspace().Proto(), pi.env.Environment(), pi.stack, plan, focus)

	if err := orchestration.Deploy(ctx, pi.env, pi.cluster, deployPlan, true, true); err != nil {
		return err
	}

	for _, obs := range pi.observers {
		obs.OnDeployment(ctx)
	}

	pi.pfw.Update(pi.stack, pi.focus, plan.IngressFragments)

	return nil
}

func (pi *updateCluster) Cleanup(_ context.Context) error {
	for _, obs := range pi.observers {
		if err := obs.Close(); err != nil {
			return err
		}
	}
	return nil
}
