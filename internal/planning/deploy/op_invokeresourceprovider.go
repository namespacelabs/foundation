// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package deploy

import (
	"context"
	"encoding/json"
	"fmt"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"k8s.io/utils/strings/slices"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/protos"
	internalres "namespacelabs.dev/foundation/internal/resources"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	runtimepb "namespacelabs.dev/foundation/schema/runtime"
	"namespacelabs.dev/foundation/std/execution"
	"namespacelabs.dev/foundation/std/resources"
	"namespacelabs.dev/foundation/std/tasks"
	"namespacelabs.dev/go-ids"
)

func register_OpInvokeResourceProvider() {
	execution.Compile[*resources.OpInvokeResourceProvider](func(ctx context.Context, inputs []*schema.SerializedInvocation) ([]*schema.SerializedInvocation, error) {
		return tasks.Return(ctx, tasks.Action("deploy.compile-invoke-resource"), func(ctx context.Context) ([]*schema.SerializedInvocation, error) {
			cluster, err := execution.Get(ctx, runtime.ClusterNamespaceInjection)
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

				spec := runtime.DeployableSpec{
					// ErrorLocation: resource.ProviderPackage.Location,

					PackageName: invoke.BinaryRef.AsPackageName(),
					Class:       schema.DeployableClass_MANUAL, // Don't emit deployment events.
					Id:          id,
					Name:        "provider",
					Description: fmt.Sprintf("Ensure resource: %s", invoke.ResourceInstanceId),

					InhibitPersistentRuntimeConfig: true,
					Resources:                      invoke.Dependency,
					SetContainerField: []*runtimepb.SetContainerField{
						// Resources are passed in as flags to minimize the number of k8s resources that are created.
						// XXX security validate this.
						{SetArg: []*runtimepb.SetContainerField_SetValue{{Key: "--resources", Value: runtimepb.SetContainerField_RESOURCE_CONFIG}}},
					},

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
	})
}

type RawJSONObject map[string]any

func BuildResourceMap(ctx context.Context, dependencies []*resources.ResourceDependency) (map[string]RawJSONObject, error) {
	return tasks.Return(ctx, tasks.Action("deploy.build-resource-map"), func(ctx context.Context) (map[string]RawJSONObject, error) {
		inputs, err := execution.Get(ctx, execution.InputsInjection)
		if err != nil {
			return nil, err
		}

		resourceData := map[string]RawJSONObject{}

		var missing []string
		for _, dep := range dependencies {
			input, ok := inputs[dep.ResourceInstanceId]
			if ok {
				raw, err := protoAsGenericJson(input.Message)
				if err != nil {
					return nil, err
				}

				resourceData[dep.GetResourceRef().Canonical()] = raw
			} else {
				missing = append(missing, dep.GetResourceRef().Canonical())
			}
		}

		if len(missing) > 0 {
			return nil, fnerrors.InvocationError("missing required resources: %v", missing)
		}

		return resourceData, nil
	})
}

func protoAsGenericJson(msg proto.Message) (RawJSONObject, error) {
	serialized, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(msg)
	if err != nil {
		return nil, err
	}

	var m RawJSONObject
	if err := json.Unmarshal(serialized, &m); err != nil {
		return nil, err
	}

	return m, nil
}
