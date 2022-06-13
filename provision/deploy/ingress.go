// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package deploy

import (
	"context"

	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/tasks"
)

type ComputeIngressResult struct {
	Fragments []*schema.IngressFragment

	rootenv ops.Environment
	stack   *schema.Stack
}

func ComputeIngress(rootenv ops.Environment, stack *schema.Stack, plans compute.Computable[[]*schema.IngressFragmentPlan], allocate bool) compute.Computable[*ComputeIngressResult] {
	return &computeIngress{rootenv: rootenv, env: rootenv.Proto(), stack: stack, plans: plans, allocate: allocate}
}

func PlanIngressDeployment(c compute.Computable[*ComputeIngressResult]) compute.Computable[runtime.DeploymentState] {
	return compute.Transform(c, func(ctx context.Context, res *ComputeIngressResult) (runtime.DeploymentState, error) {
		return runtime.For(ctx, res.rootenv).PlanIngress(ctx, res.stack, res.Fragments)
	})
}

type computeIngress struct {
	rootenv  ops.Environment
	env      *schema.Environment
	stack    *schema.Stack
	plans    compute.Computable[[]*schema.IngressFragmentPlan]
	allocate bool // Actually fetch SSL certificates etc.

	compute.LocalScoped[*ComputeIngressResult]
}

func (ci *computeIngress) Action() *tasks.ActionEvent { return tasks.Action("deploy.compute-ingress") }
func (ci *computeIngress) Inputs() *compute.In {
	return compute.Inputs().Indigestible("rootenv", ci.rootenv).Proto("env", ci.env).Proto("stack", ci.stack).Computable("plans", ci.plans)
}
func (ci *computeIngress) Output() compute.Output {
	return compute.Output{NotCacheable: true}
}
func (ci *computeIngress) Compute(ctx context.Context, deps compute.Resolved) (*ComputeIngressResult, error) {
	allFragments, err := computeDeferredIngresses(ctx, ci.env, ci.stack)
	if err != nil {
		return nil, err
	}

	plans := compute.MustGetDepValue(deps, ci.plans, "plans")
	for _, plan := range plans {
		sch := ci.stack.GetServer(schema.PackageName(plan.GetIngressFragment().GetOwner()))
		if sch == nil {
			return nil, fnerrors.BadInputError("%s: not present in the stack", plan.GetIngressFragment().GetOwner())
		}

		attached, err := runtime.AttachComputedDomains(ctx, ci.env, sch, plan.GetIngressFragment(), plan.AllocatedName)
		if err != nil {
			return nil, err
		}

		allFragments = append(allFragments, attached...)
	}

	var fragments []*schema.IngressFragment
	// XXX parallelism
	for _, fragment := range allFragments {
		sch := ci.stack.GetServer(schema.PackageName(fragment.Owner))
		if sch == nil {
			return nil, fnerrors.BadInputError("%s: not present in the stack", fragment.Owner)
		}

		if ci.allocate {
			fragment.Domain, err = runtime.MaybeAllocateDomainCertificate(ctx, sch, fragment.Domain)
			if err != nil {
				return nil, err
			}
			fragments = append(fragments, fragment)
		}

		fragments = append(fragments, fragment)
	}

	return &ComputeIngressResult{
		rootenv:   ci.rootenv,
		stack:     ci.stack,
		Fragments: fragments,
	}, nil
}

func computeDeferredIngresses(ctx context.Context, env *schema.Environment, stack *schema.Stack) ([]*schema.IngressFragment, error) {
	var fragments []*schema.IngressFragment

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
