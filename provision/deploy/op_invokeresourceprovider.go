// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package deploy

import (
	"context"
	"encoding/json"
	"fmt"

	"k8s.io/utils/strings/slices"
	"namespacelabs.dev/foundation/engine/ops"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/protos"
	internalres "namespacelabs.dev/foundation/internal/resources"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/resources"
	"namespacelabs.dev/go-ids"
)

func register_OpInvokeResourceProvider() {
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

			args := append(slices.Clone(invoke.BinaryConfig.Args), fmt.Sprintf("--intent=%s", invoke.SerializedIntentJson))

			// Resources are passed in as flags to minimize the number of k8s resources that are created.
			// XXX security validate this.

			if len(invoke.Dependency) > 0 {
				resourceData, err := BuildResourceMap(ctx, invoke.Dependency)
				if err != nil {
					return nil, err
				}

				serializedResourceData, err := json.Marshal(resourceData)
				if err != nil {
					return nil, err
				}

				args = append(args, fmt.Sprintf("--resources=%s", serializedResourceData))
			}

			spec := runtime.DeployableSpec{
				// ErrorLocation: resource.ProviderPackage.Location,

				PackageName: invoke.BinaryRef.AsPackageName(),
				Class:       schema.DeployableClass_MANUAL, // Don't emit deployment events.
				Id:          id,
				Name:        "provider",
				Description: fmt.Sprintf("Ensure resource: %s", invoke.ResourceInstanceId),

				MainContainer: runtime.ContainerRunOpts{
					Image:   imageID,
					Command: invoke.BinaryConfig.Command,
					Args:    args,
					Env:     invoke.BinaryConfig.Env,
				},
			}

			plan, err := planner.PlanDeployment(ctx, runtime.DeploymentSpec{
				Specs: []runtime.DeployableSpec{spec},
			})
			if err != nil {
				return nil, err
			}

			ops = append(ops, plan.Definitions...)

			ops = append(ops, &schema.SerializedInvocation{
				Description: fmt.Sprintf("Wait for Resource (%s:%s)", invoke.ResourceClass.PackageName, invoke.ResourceClass.Name),
				Impl: protos.WrapAnyOrDie(&internalres.OpWaitForProviderResults{
					ResourceInstanceId: invoke.ResourceInstanceId,
					Deployable:         runtime.DeployableToProto(spec),
					ResourceClass:      invoke.ResourceClass,
					InstanceTypeSource: invoke.InstanceTypeSource,
				}),
				Order: &schema.ScheduleOrder{
					SchedCategory:      []string{resources.ResourceInstanceCategory(invoke.ResourceInstanceId)},
					SchedAfterCategory: []string{runtime.DeployableCategoryID(id)},
				},
			})
		}

		return ops, nil
	})
}

func BuildResourceMap(ctx context.Context, dependencies []*resources.ResourceDependency) (map[string]any, error) {
	inputs, err := ops.Get(ctx, ops.InputsInjection)
	if err != nil {
		return nil, err
	}

	resourceData := map[string]any{}

	var missing []string
	for _, dep := range dependencies {
		input, ok := inputs[dep.ResourceInstanceId]
		if ok {
			resourceData[dep.GetResourceRef().Canonical()] = input.OriginalJSON
		} else {
			missing = append(missing, dep.GetResourceRef().Canonical())
		}
	}

	if len(missing) > 0 {
		return nil, fnerrors.InvocationError("missing required resources: %v", missing)
	}

	return resourceData, nil
}
