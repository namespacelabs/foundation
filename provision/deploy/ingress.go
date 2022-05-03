// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package deploy

import (
	"context"

	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func PlanIngress(rootenv ops.Environment, env *schema.Environment, stack *schema.Stack) compute.Computable[*ingressResult] {
	return &computeIngress{rootenv: rootenv, env: env, stack: stack}
}

func ComputeIngress(ctx context.Context, env *schema.Environment, stack *schema.Stack) ([]runtime.DeferredIngress, error) {
	var fragments []runtime.DeferredIngress

	// XXX parallelism.
	for _, srv := range stack.Entry {
		frags, err := runtime.ComputeIngress(ctx, env, srv, stack.Endpoint)
		if err != nil {
			return nil, err
		}
		fragments = append(fragments, frags...)
	}

	return fragments, nil
}

type computeIngress struct {
	rootenv ops.Environment
	env     *schema.Environment
	stack   *schema.Stack

	compute.LocalScoped[*ingressResult]
}

type ingressResult struct {
	DeploymentState runtime.DeploymentState
	Fragments       []*schema.IngressFragment
}

func (ci *computeIngress) Action() *tasks.ActionEvent { return tasks.Action("deploy.compute-ingress") }
func (ci *computeIngress) Inputs() *compute.In {
	return compute.Inputs().Indigestible("rootenv", ci.rootenv).Proto("env", ci.env).Proto("stack", ci.stack)
}
func (ci *computeIngress) Compute(ctx context.Context, deps compute.Resolved) (*ingressResult, error) {
	deferred, err := ComputeIngress(ctx, ci.env, ci.stack)
	if err != nil {
		return nil, err
	}

	var fragments []*schema.IngressFragment
	// XXX parallelism
	for _, d := range deferred {
		fragment, err := d.Allocate(ctx)
		if err != nil {
			return nil, err
		}
		fragments = append(fragments, fragment)
	}

	ds, err := runtime.For(ctx, ci.rootenv).PlanIngress(ctx, ci.stack, fragments)
	if err != nil {
		return nil, err
	}

	return &ingressResult{DeploymentState: ds, Fragments: fragments}, nil
}
