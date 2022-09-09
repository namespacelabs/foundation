// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package devworkflow

import (
	"context"
	"fmt"

	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/orchestration"
	"namespacelabs.dev/foundation/internal/runtime/endpointfwd"
	"namespacelabs.dev/foundation/languages"
	"namespacelabs.dev/foundation/provision/deploy"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/planning"
	"namespacelabs.dev/foundation/workspace/compute"
)

type updateCluster struct {
	env       planning.Context
	cluster   runtime.Cluster
	observers []languages.DevObserver

	plan  compute.Computable[*deploy.Plan]
	stack *schema.Stack
	focus []schema.PackageName

	pfw *endpointfwd.PortForward
}

func newUpdateCluster(env planning.Context, cluster runtime.Cluster, stack *schema.Stack, focus []schema.PackageName, observers []languages.DevObserver, plan compute.Computable[*deploy.Plan], pfw *endpointfwd.PortForward) *updateCluster {
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
	deployPlan := deploy.Serialize(pi.env.Workspace().Proto(), pi.env.Environment(), pi.stack, plan, focus.PackageNamesAsString())

	if err := orchestration.Deploy(ctx, pi.env, pi.cluster, plan.Deployer, deployPlan, true, true); err != nil {
		return err
	}

	for _, obs := range pi.observers {
		obs.OnDeployment()
	}

	pi.pfw.Update(pi.stack, pi.focus, plan.IngressFragments)

	return nil
}

func (pi *updateCluster) Cleanup(_ context.Context) error {
	return nil
}
