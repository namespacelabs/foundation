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
	"namespacelabs.dev/foundation/internal/runtime"
	runtimepb "namespacelabs.dev/foundation/library/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/std/resources"
	"namespacelabs.dev/foundation/std/tasks"
)

type resourcePlan struct {
	ResourceList resourceList

	Invocations []*schema.SerializedInvocation
	Secrets     []runtime.SecretResourceDependency
}

type serverStack interface {
	GetServerProto(srv schema.PackageName) (*schema.Server, bool)
	GetEndpoints(srv schema.PackageName) ([]*schema.Endpoint, bool)
}

func planResources(ctx context.Context, planner runtime.Planner, registry registry.Manager, stack serverStack, rp resourceList) (*resourcePlan, error) {
	platforms, err := planner.TargetPlatforms(ctx)
	if err != nil {
		return nil, err
	}

	rlist := rp.Resources()

	var invocations []*InvokeResourceProvider
	var imageIDs []compute.Computable[oci.ImageID]
	var ops []*schema.SerializedInvocation
	var secrets []runtime.SecretResourceDependency

	for _, resource := range rlist {
		if resource.Provider == nil {
			if len(resource.Dependencies) != 0 {
				return nil, fnerrors.InternalError("can't set dependencies on providerless-resources")
			}

			switch {
			case parsing.IsServerResource(resource.Class.Ref):
				serverIntent := &runtimepb.ServerIntent{}
				if err := proto.Unmarshal(resource.Intent.Value, serverIntent); err != nil {
					return nil, fnerrors.InternalError("failed to unmarshal serverintent: %w", err)
				}

				target, has := stack.GetServerProto(schema.PackageName(serverIntent.PackageName))
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

				wrapped, err := anypb.New(&resources.OpHandleServerDependency{
					ResourceInstanceId: resource.ID,
					ServerConfig:       makeServerConfig(stack, target),
					Deployable:         runtime.DeployableToProto(target),
				})
				if err != nil {
					return nil, err
				}

				si.Impl = wrapped

				ops = append(ops, si)
			}

			continue // Nothing to do re: provider.
		}

		provider := resource.Provider.Spec

		if provider.PrepareWith != nil {
			return nil, fnerrors.InternalError("unimplemented")
		}

		initializer := provider.InitializedWith
		if initializer.RequiresKeys || initializer.Snapshots != nil || initializer.Inject != nil {
			return nil, fnerrors.InternalError("bad resource provider initialization: unsupported inputs")
		}

		if len(resource.ParentContexts) == 0 {
			return nil, fnerrors.InternalError("%s: resource is missing a context", resource.ID)
		}

		// Any of the contexts should be valid to load the binary, as all of them refer to this resources.
		sealedCtx := resource.ParentContexts[0]

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
		invocations = append(invocations, &InvokeResourceProvider{
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
	}

	builtImages, err := compute.GetValue(ctx, compute.Collect(tasks.Action("resource-provider.build-image"), imageIDs...))
	if err != nil {
		return nil, err
	}

	for k, invocation := range invocations {
		invocation.BinaryImageId = builtImages[k].Value

		theseOps, err := PlanResourceProviderInvocation(ctx, planner, invocation)
		if err != nil {
			return nil, err
		}

		ops = append(ops, theseOps...)
	}

	return &resourcePlan{ResourceList: rp, Invocations: ops, Secrets: secrets}, nil
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
	Secrets              []runtime.SecretResourceDependency
}

type ownedResourceInstances struct {
	Dependencies []*resources.ResourceDependency
	Secrets      []runtime.SecretResourceDependency
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

	rp.perOwnerResources[owner.PackageRef().Canonical()] = ownedResourceInstances{instance.Dependencies, instance.Secrets}

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
			instance.Dependencies = append(instance.Dependencies, &resources.ResourceDependency{
				ResourceRef:        res.ResourceRef,
				ResourceClass:      res.Spec.Class.Ref,
				ResourceInstanceId: scopedID,
			})
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
			secretIntent := &runtimepb.SecretIntent{}
			if err := proto.Unmarshal(dep.Spec.Source.Intent.Value, secretIntent); err != nil {
				return nil, nil, fnerrors.InternalError("failed to unmarshal serverintent: %w", err)
			}

			ref, err := schema.StrictParsePackageRef(secretIntent.Ref)
			if err != nil {
				return nil, nil, fnerrors.BadInputError("failed to resolve reference: %w", err)
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
