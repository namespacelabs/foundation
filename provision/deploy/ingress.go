// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package deploy

import (
	"context"

	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/executor"
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

func ComputeIngress(rootenv ops.Environment, stack *schema.Stack, plans compute.Computable[[]*schema.IngressFragment], allocate bool) compute.Computable[*ComputeIngressResult] {
	return &computeIngress{rootenv: rootenv, stack: stack, fragments: plans, allocate: allocate}
}

func PlanIngressDeployment(c compute.Computable[*ComputeIngressResult]) compute.Computable[runtime.DeploymentState] {
	return compute.Transform(c, func(ctx context.Context, res *ComputeIngressResult) (runtime.DeploymentState, error) {
		return runtime.For(ctx, res.rootenv).PlanIngress(ctx, res.stack, res.Fragments)
	})
}

type computeIngress struct {
	rootenv   ops.Environment
	stack     *schema.Stack
	fragments compute.Computable[[]*schema.IngressFragment]
	allocate  bool // Actually fetch SSL certificates etc.

	compute.LocalScoped[*ComputeIngressResult]
}

func (ci *computeIngress) Action() *tasks.ActionEvent { return tasks.Action("deploy.compute-ingress") }
func (ci *computeIngress) Inputs() *compute.In {
	return compute.Inputs().
		Indigestible("rootenv", ci.rootenv).
		Proto("stack", ci.stack).
		Computable("fragments", ci.fragments)
}
func (ci *computeIngress) Output() compute.Output {
	return compute.Output{NotCacheable: true}
}
func (ci *computeIngress) Compute(ctx context.Context, deps compute.Resolved) (*ComputeIngressResult, error) {
	allFragments, err := computeDeferredIngresses(ctx, ci.rootenv, ci.stack)
	if err != nil {
		return nil, err
	}

	computed := compute.MustGetDepValue(deps, ci.fragments, "fragments")
	allFragments = append(allFragments, computed...)

	eg := executor.New(ctx, "compute.ingress")
	for _, fragment := range allFragments {
		fragment := fragment // Close fragment.

		eg.Go(func(ctx context.Context) error {
			sch := ci.stack.GetServer(schema.PackageName(fragment.Owner))
			if sch == nil {
				return fnerrors.BadInputError("%s: not present in the stack", fragment.Owner)
			}

			if ci.allocate {
				fragment.DomainCertificate, err = runtime.MaybeAllocateDomainCertificate(ctx, ci.rootenv.Proto(), sch, fragment.Domain)
				if err != nil {
					return err
				}
			}

			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	return &ComputeIngressResult{
		rootenv:   ci.rootenv,
		stack:     ci.stack,
		Fragments: allFragments,
	}, nil
}

func computeDeferredIngresses(ctx context.Context, env ops.Environment, stack *schema.Stack) ([]*schema.IngressFragment, error) {
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
