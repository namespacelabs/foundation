// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package deploy

import (
	"context"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/dynamicpb"
	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/build/binary"
	"namespacelabs.dev/foundation/engine/compute"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnerrors/multierr"
	"namespacelabs.dev/foundation/internal/uniquestrings"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/std/resources"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func planResources(ctx context.Context, sealedCtx pkggraph.SealedContext, planner runtime.Planner, rp resourceList) ([]*schema.SerializedInvocation, error) {
	platforms, err := planner.TargetPlatforms(ctx)
	if err != nil {
		return nil, err
	}

	var imageIDs []compute.Computable[oci.ImageID]
	var invocations []*resources.OpInvokeResourceProvider

	for _, resource := range rp.Resources() {
		provider := resource.Provider.Spec

		if provider.PrepareWith == nil {
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

		prepared, err := binary.PlanBinary(ctx, sealedCtx, sealedCtx, pkg.Location, bin, binary.BuildImageOpts{
			UsePrebuilts: true,
			Platforms:    platforms,
		})
		if err != nil {
			return nil, err
		}

		imageID, _, err := ensureImage(ctx, sealedCtx, prepared.Plan)
		if err != nil {
			return nil, err
		}

		imageIDs = append(imageIDs, imageID)

		invocations = append(invocations, &resources.OpInvokeResourceProvider{
			ResourceInstanceId:   resource.ID,
			BinaryRef:            initializer.BinaryRef,
			BinaryConfig:         bin.Config,
			ResourceClass:        resource.Class.Spec,
			ResourceProvider:     provider,
			InstanceTypeSource:   resource.Class.InstanceType.Sources,
			SerializedIntentJson: resource.JSONSerializedIntent,
		})
	}

	builtImages, err := compute.GetValue(ctx, compute.Collect(tasks.Action("resource-provider.build-image"), imageIDs...))
	if err != nil {
		return nil, err
	}

	var ops []*schema.SerializedInvocation
	for k, img := range builtImages {
		invocations[k].BinaryImageId = img.Value.RepoAndDigest()

		wrapped, err := anypb.New(invocations[k])
		if err != nil {
			return nil, err
		}

		ops = append(ops, &schema.SerializedInvocation{
			Description: "Invoke Resource Provider",
			Impl:        wrapped,
		})
	}

	return ops, nil
}

type resourceList struct {
	resourceIDs uniquestrings.List

	resources map[string]resourceInstance
}

type resourceInstance struct {
	ID                   string
	Class                pkggraph.ResourceClass
	Provider             pkggraph.ResourceProvider
	Intent               *anypb.Any
	JSONSerializedIntent []byte
}

func (rp *resourceList) Resources() []resourceInstance {
	var resources []resourceInstance
	for _, resourceID := range rp.resourceIDs.Strings() {
		resource := rp.resources[resourceID]
		resources = append(resources, resource)
	}
	return resources
}

func (rp *resourceList) checkAddMultiple(ctx context.Context, pl pkggraph.PackageLoader, resourceRefs ...*schema.PackageRef) error {
	var errs []error
	for _, ref := range resourceRefs {
		if err := rp.checkAdd(ctx, pl, ref); err != nil {
			errs = append(errs, err)
		}
	}

	return multierr.New(errs...)
}

func (rp *resourceList) checkAdd(ctx context.Context, pl pkggraph.PackageLoader, resourceRef *schema.PackageRef) error {
	resourceID := resources.ResourceID(resourceRef)

	if !rp.resourceIDs.Add(resourceID) {
		return nil
	}

	pkg, err := pl.LoadByName(ctx, resourceRef.AsPackageName())
	if err != nil {
		return err
	}

	resource := pkg.LookupResourceInstance(resourceRef.Name)
	if resource == nil {
		return fnerrors.InternalError("%s: missing resource", resourceRef.Canonical())
	}

	if rp.resources == nil {
		rp.resources = map[string]resourceInstance{}
	}

	// XXX add resources recursively.
	instance := resourceInstance{
		ID:       resourceID,
		Class:    resource.Class,
		Provider: resource.Provider,
		Intent:   resource.Spec.Intent,
	}

	if instance.Intent != nil {
		out := dynamicpb.NewMessage(resource.Class.IntentType.Descriptor).Interface()

		if err := proto.Unmarshal(instance.Intent.Value, out); err != nil {
			return fnerrors.InternalError("%s: failed to unmarshal intent: %w", resourceRef.Canonical(), err)
		}

		// json.Marshal is not capable of serializing a dynamicpb.
		serialized, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(out)
		if err != nil {
			return fnerrors.InternalError("%s: failed to marshal intent to json: %w", resourceRef.Canonical(), err)
		}

		instance.JSONSerializedIntent = serialized
	}

	rp.resources[instance.ID] = instance
	return nil
}
