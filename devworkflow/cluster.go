// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package devworkflow

import (
	"context"

	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/runtime/endpointfwd"
	"namespacelabs.dev/foundation/languages"
	"namespacelabs.dev/foundation/provision/deploy"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/compute"
)

type updateCluster struct {
	env       workspace.WorkspaceEnvironment
	observers []languages.DevObserver

	plan  compute.Computable[*deploy.Plan]
	stack *schema.Stack
	focus []schema.PackageName

	pfw *endpointfwd.PortForward
}

func newPortFwd(obs *stackState, env ops.Environment, localaddr string) *endpointfwd.PortForward {
	pfw := &endpointfwd.PortForward{
		Env:       env,
		LocalAddr: localaddr,
	}

	pfw.OnAdd = func(endpoint *schema.Endpoint, localPort uint) {
		obs.updateStack(func(stack *Stack) *Stack {
			for _, fwd := range stack.ForwardedPort {
				if fwd.Endpoint == endpoint {
					fwd.LocalPort = int32(localPort)
					return stack
				}
			}

			stack.ForwardedPort = append(stack.ForwardedPort, &ForwardedPort{
				Endpoint:      endpoint,
				ContainerPort: endpoint.GetPort().GetContainerPort(),
				LocalPort:     int32(localPort),
			})
			return stack
		})
	}

	pfw.OnDelete = func(unused []*schema.Endpoint) {
		obs.updateStack(func(stack *Stack) *Stack {
			var portFwds []*ForwardedPort
			for _, fwd := range stack.ForwardedPort {
				filtered := false
				for _, endpoint := range unused {
					if fwd.Endpoint == endpoint {
						filtered = true
						break
					}
				}
				if !filtered {
					portFwds = append(portFwds, fwd)
				}
			}

			stack.ForwardedPort = portFwds
			return stack
		})
	}

	pfw.OnUpdate = func() {
		obs.parent.setSticky(pfw.Render())
	}

	return pfw
}

func newUpdateCluster(obs *stackState, env workspace.WorkspaceEnvironment, stack *schema.Stack, focus []schema.PackageName, observers []languages.DevObserver, plan compute.Computable[*deploy.Plan]) *updateCluster {
	return &updateCluster{
		env:       env,
		observers: observers,
		plan:      plan,
		stack:     stack,
		focus:     focus,
		pfw:       newPortFwd(obs, env, obs.parent.localHostname),
	}
}

func (pi *updateCluster) Inputs() *compute.In {
	return compute.Inputs().Computable("plan", pi.plan).Proto("stack", pi.stack).JSON("focus", pi.focus)
}

func (pi *updateCluster) Updated(ctx context.Context, deps compute.Resolved) error {
	plan := compute.GetDepValue(deps, pi.plan, "plan")

	waiters, err := plan.Deployer.Execute(ctx, runtime.TaskServerDeploy, pi.env)
	if err != nil {
		return err
	}

	if err := deploy.Wait(ctx, pi.env, waiters); err != nil {
		return err
	}

	for _, obs := range pi.observers {
		obs.OnDeployment()
	}

	pi.pfw.Update(ctx, pi.stack, pi.focus, plan.IngressFragments)

	return nil
}

func (pi *updateCluster) Cleanup(_ context.Context) error {
	return pi.pfw.Cleanup()
}
