// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package deploy

import (
	"context"
	"fmt"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/dynamicpb"
	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/build/assets"
	"namespacelabs.dev/foundation/build/binary"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/artifacts/registry"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnerrors/multierr"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/internal/planning"
	"namespacelabs.dev/foundation/internal/uniquestrings"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/std/resources"
	stdruntime "namespacelabs.dev/foundation/std/runtime"
	"namespacelabs.dev/foundation/std/tasks"
)

type resourcePlan struct {
	Invocations []*schema.SerializedInvocation
	Secrets     []runtime.SecretResource
}

func planResources(ctx context.Context, sealedCtx pkggraph.SealedContext, planner runtime.Planner, registry registry.Manager, stack *planning.Stack, rp resourceList) (*resourcePlan, error) {
	platforms, err := planner.TargetPlatforms(ctx)
	if err != nil {
		return nil, err
	}

	rlist := rp.Resources()

	var invocations []*InvokeResourceProvider
	var imageIDs []compute.Computable[oci.ImageID]
	var ops []*schema.SerializedInvocation
	var secrets []runtime.SecretResource

	for _, resource := range rlist {
		if resource.Provider == nil {
			if len(resource.Dependencies) != 0 {
				return nil, fnerrors.InternalError("can't set dependencies on providerless-resources")
			}

			switch {
			case parsing.IsServerResource(resource.Class.Ref):
				serverIntent := &stdruntime.ServerIntent{}
				if err := proto.Unmarshal(resource.Intent.Value, serverIntent); err != nil {
					return nil, fnerrors.InternalError("failed to unmarshal serverintent: %w", err)
				}

				si := &schema.SerializedInvocation{
					Description: "Capture Runtime Config",
					Order: &schema.ScheduleOrder{
						SchedCategory: []string{
							resources.ResourceInstanceCategory(resource.ID),
						},
					},
				}

				wrapped, err := anypb.New(&resources.OpCaptureServerConfig{
					ResourceInstanceId: resource.ID,
					Server:             MakeServerConfig(stack, schema.PackageName(serverIntent.PackageName)),
				})
				if err != nil {
					return nil, err
				}

				si.Impl = wrapped

				ops = append(ops, si)

			case parsing.IsSecretResource(resource.Class.Ref):
				secretIntent := &stdruntime.SecretIntent{}
				if err := proto.Unmarshal(resource.Intent.Value, secretIntent); err != nil {
					return nil, fnerrors.InternalError("failed to unmarshal serverintent: %w", err)
				}

				ref, err := schema.ParsePackageRef(schema.PackageName(resource.Source.PackageName), secretIntent.Ref)
				if err != nil {
					return nil, fnerrors.BadInputError("failed to resolve reference: %w", err)
				}

				specs, err := loadSecretSpecs(ctx, sealedCtx, ref)
				if err != nil {
					return nil, err
				}

				secrets = append(secrets, runtime.SecretResource{
					ResourceID: resource.ID,
					Spec:       specs[0],
				})
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

		imageID, _, err := ensureImage(ctx, sealedCtx, registry, prepared.Plan)
		if err != nil {
			return nil, err
		}

		imageIDs = append(imageIDs, imageID)
		invocations = append(invocations, &InvokeResourceProvider{
			ResourceInstanceId:   resource.ID,
			BinaryRef:            initializer.BinaryRef,
			BinaryConfig:         bin.Config,
			ResourceClass:        resource.Class.Source,
			ResourceProvider:     provider,
			InstanceTypeSource:   resource.Class.InstanceType.Sources,
			SerializedIntentJson: resource.JSONSerializedIntent,
			Dependency:           resource.Dependencies,
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

	return &resourcePlan{Invocations: ops, Secrets: secrets}, nil
}

type resourceList struct {
	resourceIDs uniquestrings.List

	resources map[string]resourceInstance
}

type resourceInstance struct {
	ID                   string
	Source               *schema.ResourceInstance
	Class                pkggraph.ResourceClass
	Provider             *pkggraph.ResourceProvider
	Intent               *anypb.Any
	JSONSerializedIntent []byte
	Dependencies         []*resources.ResourceDependency
}

func (rp *resourceList) Resources() []resourceInstance {
	var resources []resourceInstance
	for _, resourceID := range rp.resourceIDs.Strings() {
		resource := rp.resources[resourceID]
		resources = append(resources, resource)
	}
	return resources
}

func (rp *resourceList) checkAddMultiple(ctx context.Context, instances ...pkggraph.ResourceInstance) error {
	var errs []error
	for _, instance := range instances {
		resourceID := resources.ResourceID(instance.Name)

		if err := rp.checkAddResource(ctx, resourceID, instance.Spec); err != nil {
			errs = append(errs, err)
		}
	}

	return multierr.New(errs...)
}

func (rp *resourceList) checkAddResource(ctx context.Context, resourceID string, resource pkggraph.ResourceSpec) error {
	if !rp.resourceIDs.Add(resourceID) {
		return nil
	}

	if rp.resources == nil {
		rp.resources = map[string]resourceInstance{}
	}

	// XXX add resources recursively.
	instance := resourceInstance{
		ID:       resourceID,
		Source:   resource.Source,
		Class:    resource.Class,
		Provider: resource.Provider,
		Intent:   resource.Source.Intent,
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

	// Add static resources required by providers.
	for _, res := range inputs {
		scopedID := fmt.Sprintf("%s;%s:%s", resourceID, res.Name.PackageName, res.Name.Name)

		if err := rp.checkAddResource(ctx, scopedID, res.Spec); err != nil {
			return err
		}

		instance.Dependencies = append(instance.Dependencies, &resources.ResourceDependency{
			ResourceRef:        res.Name,
			ResourceClass:      res.Spec.Class.Ref,
			ResourceInstanceId: scopedID,
		})
	}

	rp.resources[instance.ID] = instance

	return nil
}
