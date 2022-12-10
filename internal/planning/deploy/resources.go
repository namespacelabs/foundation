// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package deploy

import (
	"context"
	"fmt"
	"strings"

	"golang.org/x/exp/slices"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/dynamicpb"
	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/framework/rpcerrors/multierr"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/artifacts/registry"
	"namespacelabs.dev/foundation/internal/build/assets"
	"namespacelabs.dev/foundation/internal/build/binary"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/internal/planning/invocation"
	"namespacelabs.dev/foundation/internal/planning/tool"
	"namespacelabs.dev/foundation/internal/planning/tool/protocol"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/std/resources"
	"namespacelabs.dev/foundation/std/tasks"
)

type resourcePlan struct {
	ResourceList resourceList

	PlannedResources     []plannedResource
	ExecutionInvocations []*schema.SerializedInvocation
	Secrets              []runtime.SecretResourceDependency
}

type plannedResource struct {
	runtime.PlannedResource
	Invocations []*schema.SerializedInvocation
}

type serverStack interface {
	GetServerProto(srv schema.PackageName) (*schema.Server, bool)
	GetEndpoints(srv schema.PackageName) ([]*schema.Endpoint, bool)
}

func planResources(ctx context.Context, modules pkggraph.Modules, planner runtime.Planner, registry registry.Manager, stack serverStack, rp resourceList) (*resourcePlan, error) {
	platforms, err := planner.TargetPlatforms(ctx)
	if err != nil {
		return nil, err
	}

	rlist := rp.Resources()

	type resourcePlanInvocation struct {
		Env                  cfg.Context
		Source               schema.PackageName
		ResourceInstanceID   string
		ResourceSource       *schema.ResourceInstance
		ResourceClass        *schema.ResourceClass
		Invocation           *invocation.Invocation
		Intent               *anypb.Any
		SerializedIntentJson []byte
		Image                oci.ResolvableImage
	}

	var executionInvocations []*InvokeResourceProvider
	var planningInvocations []resourcePlanInvocation
	var planningImages []compute.Computable[oci.ResolvableImage]
	var imageIDs []compute.Computable[oci.ImageID]

	plan := &resourcePlan{
		ResourceList: rp,
	}

	for _, resource := range rlist {
		if resource.Provider == nil {
			if len(resource.Dependencies) != 0 {
				return nil, fnerrors.InternalError("can't set dependencies on providerless-resources")
			}

			switch {
			case parsing.IsServerResource(resource.Class.Ref):
				if err := pkggraph.ValidateFoundation("runtime resources", parsing.Version_LibraryIntentsChanged, pkggraph.ModuleFromModules(modules)); err != nil {
					return nil, err
				}

				serverIntent := &schema.PackageRef{}
				if err := proto.Unmarshal(resource.Intent.Value, serverIntent); err != nil {
					return nil, fnerrors.InternalError("failed to unmarshal serverintent: %w", err)
				}

				target, has := stack.GetServerProto(serverIntent.AsPackageName())
				if !has {
					return nil, fnerrors.InternalError("%s: target server is not in the stack", serverIntent.PackageName)
				}

				si := &schema.SerializedInvocation{
					Description: "Capture Runtime Config",
					Order: &schema.ScheduleOrder{
						SchedCategory: []string{
							resources.ResourceInstanceCategory(resource.ID),
						},
						SchedAfterCategory: []string{
							runtime.OwnedByDeployable(target),
						},
					},
				}

				wrapped, err := anypb.New(&resources.OpCaptureServerConfig{
					ResourceInstanceId: resource.ID,
					ServerConfig:       makeServerConfig(stack, target),
					Deployable:         runtime.DeployableToProto(target),
				})
				if err != nil {
					return nil, err
				}

				si.Impl = wrapped

				plan.ExecutionInvocations = append(plan.ExecutionInvocations, si)
			}

			continue // Nothing to do re: provider.
		}

		if len(resource.ParentContexts) == 0 {
			return nil, fnerrors.InternalError("%s: resource is missing a context", resource.ID)
		}

		// Any of the contexts should be valid to load the binary, as all of them refer to this resources.
		sealedCtx := resource.ParentContexts[0]

		provider := resource.Provider.Spec

		if provider.PrepareWith != nil {
			var errs []error
			if len(resource.Dependencies) > 0 {
				errs = append(errs, fnerrors.New("providers with prepareWith don't support dependencies yet"))
			}

			if len(resource.Secrets) > 0 {
				errs = append(errs, fnerrors.New("providers with prepareWith don't support secrets yet"))
			}

			inv, err := invocation.Make(ctx, sealedCtx, sealedCtx, nil, provider.PrepareWith)
			if err != nil {
				errs = append(errs, err)
			}

			if err := multierr.New(errs...); err != nil {
				return nil, err
			}

			planningImages = append(planningImages, inv.Image)
			planningInvocations = append(planningInvocations, resourcePlanInvocation{
				Env:                  sealedCtx,
				Source:               schema.PackageName(resource.Source.PackageName),
				ResourceInstanceID:   resource.ID,
				ResourceSource:       resource.Source,
				ResourceClass:        resource.Class.Source,
				Invocation:           inv,
				Intent:               resource.Intent,
				SerializedIntentJson: resource.JSONSerializedIntent,
			})
		} else if provider.InitializedWith != nil {
			initializer := provider.InitializedWith
			if initializer.RequiresKeys || initializer.Snapshots != nil || initializer.Inject != nil {
				return nil, fnerrors.InternalError("bad resource provider initialization: unsupported inputs")
			}

			pkg, bin, err := pkggraph.LoadBinary(ctx, sealedCtx, initializer.BinaryRef)
			if err != nil {
				return nil, err
			}

			prepared, err := binary.PlanBinary(ctx, sealedCtx, sealedCtx, pkg.Location, bin, assets.AvailableBuildAssets{}, binary.BuildImageOpts{
				UsePrebuilts: true,
				Platforms:    platforms,
			})
			if err != nil {
				return nil, err
			}

			poster, err := ensureImage(ctx, sealedCtx, registry, prepared.Plan)
			if err != nil {
				return nil, err
			}

			imageIDs = append(imageIDs, poster.ImageID)
			executionInvocations = append(executionInvocations, &InvokeResourceProvider{
				ResourceInstanceId:   resource.ID,
				BinaryRef:            initializer.BinaryRef,
				BinaryConfig:         bin.Config,
				ResourceClass:        resource.Class.Source,
				ResourceProvider:     provider,
				InstanceTypeSource:   resource.Class.InstanceType.Sources,
				SerializedIntentJson: resource.JSONSerializedIntent,
				ResourceDependencies: resource.Dependencies,
				SecretResources:      resource.Secrets,
			})
		} else {
			return nil, fnerrors.InternalError("%s: an initializer is missing", resource.ID)
		}
	}

	if len(executionInvocations) > 0 {
		builtExecutionImages, err := compute.GetValue(ctx, compute.Collect(tasks.Action("resources.build-execution-images"), imageIDs...))
		if err != nil {
			return nil, err
		}

		for k, invocation := range executionInvocations {
			invocation.BinaryImageId = builtExecutionImages[k].Value

			theseOps, err := PlanResourceProviderInvocation(ctx, modules, planner, invocation)
			if err != nil {
				return nil, err
			}

			plan.ExecutionInvocations = append(plan.ExecutionInvocations, theseOps...)
		}
	}

	if len(planningInvocations) > 0 {
		builtPlanningImages, err := compute.GetValue(ctx, compute.Collect(tasks.Action("resources.build-planning-images"), planningImages...))
		if err != nil {
			return nil, err
		}

		for k := range planningInvocations {
			planningInvocations[k].Image = builtPlanningImages[k].Value
		}

		var x []compute.Computable[*protocol.ToolResponse]
		var errs []error
		for _, planned := range planningInvocations {
			source, err := anypb.New(&protocol.ResourceInstance{
				ResourceInstanceId: planned.ResourceInstanceID,
				ResourceInstance:   planned.ResourceSource,
			})
			if err != nil {
				errs = append(errs, err)
				continue
			}

			inv, err := tool.MakeInvocationNoInjections(ctx, planned.Env, &tool.Definition{
				Source:     tool.Source{PackageName: planned.Source},
				Invocation: planned.Invocation,
			}, tool.InvokeProps{
				Event:          protocol.Lifecycle_PROVISION,
				ProvisionInput: []*anypb.Any{planned.Intent, source},
			})
			if err != nil {
				errs = append(errs, err)
			} else {
				x = append(x, inv)
			}
		}

		if err := multierr.New(errs...); err != nil {
			return nil, err
		}

		responses, err := compute.GetValue(ctx, compute.Collect(tasks.Action("resources.invoke-providers"), x...))
		if err != nil {
			return nil, err
		}

		for k, raw := range responses {
			response := raw.Value

			if response.ApplyResponse == nil {
				return nil, fnerrors.InternalError("missing apply response")
			}

			r := response.ApplyResponse
			if len(r.ServerExtension) > 0 || len(r.Extension) > 0 {
				return nil, fnerrors.InternalError("a resource planner can not return server extensions")
			}
			if len(r.InvocationSource) > 0 {
				return nil, fnerrors.InternalError("computable invocation sources not supported in this path")
			}
			if len(r.Computed) > 0 {
				return nil, fnerrors.InternalError("compute configurations not supported in this path")
			}

			plan.PlannedResources = append(plan.PlannedResources, plannedResource{
				PlannedResource: runtime.PlannedResource{
					ResourceInstanceID: planningInvocations[k].ResourceInstanceID,
					Class:              planningInvocations[k].ResourceClass,
					Instance:           r.OutputResourceInstance,
				},
				Invocations: r.Invocation,
			})
		}
	}

	return plan, nil
}

type resourceList struct {
	resources         map[string]*resourceInstance
	perOwnerResources ResourceMap
}

type ResourceMap map[string]ownedResourceInstances // The key is a canonical PackageRef.

type resourceInstance struct {
	ParentContexts []pkggraph.SealedContext

	ID                   string
	Source               *schema.ResourceInstance
	Class                pkggraph.ResourceClass
	Provider             *pkggraph.ResourceProvider
	Intent               *anypb.Any
	JSONSerializedIntent []byte
	Dependencies         []*resources.ResourceDependency
	PlannedDependencies  []*resources.ResourceDependency
	Secrets              []runtime.SecretResourceDependency
}

type ownedResourceInstances struct {
	Dependencies        []*resources.ResourceDependency
	PlannedDependencies []*resources.ResourceDependency
	Secrets             []runtime.SecretResourceDependency
}

func (rp *resourceList) Resources() []*resourceInstance {
	var resources []*resourceInstance
	for _, res := range rp.resources {
		resources = append(resources, res)
	}
	slices.SortFunc(resources, func(a, b *resourceInstance) bool {
		return strings.Compare(a.ID, b.ID) < 0
	})
	return resources
}

type resourceOwner interface {
	SealedContext() pkggraph.SealedContext
	PackageRef() *schema.PackageRef
}

func (rp *resourceList) checkAddOwnedResources(ctx context.Context, owner resourceOwner, instances []pkggraph.ResourceInstance) error {
	var instance resourceInstance

	if err := rp.checkAddTo(ctx, owner.SealedContext(), "", instances, &instance); err != nil {
		return err
	}

	if rp.perOwnerResources == nil {
		rp.perOwnerResources = ResourceMap{}
	}

	rp.perOwnerResources[owner.PackageRef().Canonical()] = ownedResourceInstances{
		instance.Dependencies,
		instance.PlannedDependencies,
		instance.Secrets,
	}

	return nil
}

func (rp *resourceList) checkAddResource(ctx context.Context, sealedCtx pkggraph.SealedContext, resourceID string, resource pkggraph.ResourceSpec) error {
	if existing, has := rp.resources[resourceID]; has {
		existing.ParentContexts = append(existing.ParentContexts, sealedCtx)
		return nil
	}

	if rp.resources == nil {
		rp.resources = map[string]*resourceInstance{}
	}

	instance := resourceInstance{
		ParentContexts: []pkggraph.SealedContext{sealedCtx},
		ID:             resourceID,
		Source:         resource.Source,
		Class:          resource.Class,
		Provider:       resource.Provider,
		Intent:         resource.Source.Intent,
	}

	if instance.Intent != nil {
		out := dynamicpb.NewMessage(resource.Class.IntentType.Descriptor).Interface()

		if err := proto.Unmarshal(instance.Intent.Value, out); err != nil {
			return fnerrors.InternalError("%s: failed to unmarshal intent: %w", resourceID, err)
		}

		// json.Marshal is not capable of serializing a dynamicpb.
		serialized, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(out)
		if err != nil {
			return fnerrors.InternalError("%s: failed to marshal intent to json: %w", resourceID, err)
		}

		instance.JSONSerializedIntent = serialized
	}

	var inputs []pkggraph.ResourceInstance
	if resource.Provider != nil {
		inputs = append(inputs, resource.Provider.Resources...)
	}

	inputs = append(inputs, resource.ResourceInputs...)

	if err := rp.checkAddTo(ctx, sealedCtx, resourceID, inputs, &instance); err != nil {
		return err
	}

	rp.resources[resourceID] = &instance
	return nil
}

func (rp *resourceList) checkAddTo(ctx context.Context, sealedCtx pkggraph.SealedContext, parentID string, inputs []pkggraph.ResourceInstance, instance *resourceInstance) error {
	regular, secrets, err := splitRegularAndSecretResources(ctx, sealedCtx, inputs)
	if err != nil {
		return err
	}

	// Add static resources required by providers.
	var errs []error
	for _, res := range regular {
		scopedID := resources.ResourceID(res.ResourceRef)
		if parentID != "" {
			scopedID = fmt.Sprintf("%s;%s", parentID, scopedID)
		}

		if err := rp.checkAddResource(ctx, sealedCtx, scopedID, res.Spec); err != nil {
			errs = append(errs, err)
		} else {
			dep := &resources.ResourceDependency{
				ResourceRef:        res.ResourceRef,
				ResourceClass:      res.Spec.Class.Ref,
				ResourceInstanceId: scopedID,
			}

			if res.Spec.Provider != nil && res.Spec.Provider.Spec.GetPrepareWith() != nil {
				instance.PlannedDependencies = append(instance.PlannedDependencies, dep)
			} else {
				instance.Dependencies = append(instance.Dependencies, dep)
			}
		}
	}

	instance.Secrets = secrets
	return multierr.New(errs...)
}

func splitRegularAndSecretResources(ctx context.Context, pl pkggraph.PackageLoader, inputs []pkggraph.ResourceInstance) ([]pkggraph.ResourceInstance, []runtime.SecretResourceDependency, error) {
	var regularDependencies []pkggraph.ResourceInstance
	var secrets []runtime.SecretResourceDependency
	for _, dep := range inputs {
		if parsing.IsSecretResource(dep.Spec.Class.Ref) {
			ref := &schema.PackageRef{}
			if err := proto.Unmarshal(dep.Spec.Source.Intent.Value, ref); err != nil {
				return nil, nil, fnerrors.InternalError("failed to unmarshal serverintent: %w", err)
			}

			specs, err := loadSecretSpecs(ctx, pl, ref)
			if err != nil {
				return nil, nil, err
			}

			secrets = append(secrets, runtime.SecretResourceDependency{
				SecretRef:   ref,
				ResourceRef: dep.ResourceRef,
				Spec:        specs[0],
			})
		} else {
			regularDependencies = append(regularDependencies, dep)
		}
	}

	return regularDependencies, secrets, nil
}
