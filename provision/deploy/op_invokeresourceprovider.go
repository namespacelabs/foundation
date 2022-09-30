// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package deploy

import (
	"context"
	"fmt"

	"namespacelabs.dev/foundation/engine/ops"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/protos"
	internalres "namespacelabs.dev/foundation/internal/resources"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/resources"
	"namespacelabs.dev/go-ids"
)

func Register_OpInvokeResourceProvider() {
	ops.Compile[*resources.OpInvokeResourceProvider](func(ctx context.Context, inputs []*schema.SerializedInvocation) ([]*schema.SerializedInvocation, error) {
		cluster, err := ops.Get(ctx, runtime.ClusterNamespaceInjection)
		if err != nil {
			return nil, err
		}

		planner := cluster.Planner()

		var ops []*schema.SerializedInvocation
		for _, input := range inputs {
			raw, err := input.Impl.UnmarshalNew()
			if err != nil {
				return nil, err
			}

			invoke := raw.(*resources.OpInvokeResourceProvider)

			imageID, err := oci.ParseImageID(invoke.BinaryImageId)
			if err != nil {
				return nil, err
			}

			id := ids.NewRandomBase32ID(8)

			spec := runtime.DeployableSpec{
				// ErrorLocation: resource.ProviderPackage.Location,

				PackageName: invoke.BinaryRef.AsPackageName(),
				Class:       schema.DeployableClass_ONESHOT,
				Id:          id,
				MainContainer: runtime.ContainerRunOpts{
					Image:   imageID,
					Command: invoke.BinaryConfig.Command,
					Args:    invoke.BinaryConfig.Args,
					Env:     invoke.BinaryConfig.Env,
				},
			}

			plan, err := planner.PlanDeployment(ctx, runtime.DeploymentSpec{
				Specs: []runtime.DeployableSpec{
					spec,
				},
			})
			if err != nil {
				return nil, err
			}

			for _, def := range plan.Definitions {
				if def.Order == nil {
					def.Order = &schema.ScheduleOrder{}
				}

				def.Order.SchedCategory = append(def.Order.SchedCategory, category(id))
				ops = append(ops, def)
			}

			ops = append(ops, &schema.SerializedInvocation{
				Description: fmt.Sprintf("Resource provider for %s:%s", invoke.ResourceClass.PackageName, invoke.ResourceClass.Name),
				Impl: protos.WrapAnyOrDie(&internalres.OpWaitForProviderResults{
					Deployable:         runtime.DeployableToProto(spec),
					ResourceClass:      invoke.ResourceClass,
					InstanceTypeSource: invoke.InstanceTypeSource,
				}),
				Order: &schema.ScheduleOrder{
					SchedAfterCategory: []string{category(id)},
				},
			})
		}

		return ops, nil
	})
}

func category(id string) string {
	return fmt.Sprintf("invocation:%s", id)
}
