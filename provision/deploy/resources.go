// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package deploy

import (
	"context"
	"fmt"

	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/build/binary"
	"namespacelabs.dev/foundation/engine/compute"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnerrors/multierr"
	"namespacelabs.dev/foundation/internal/uniquestrings"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/std/planning"
	"namespacelabs.dev/foundation/std/resources"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func PlanResources(ctx context.Context, pl pkggraph.SealedPackageLoader, env planning.Context, planner runtime.Planner, stack *provision.Stack) ([]*schema.SerializedInvocation, error) {
	var rp resourcePlanner

	var errs []error
	for _, ps := range stack.Servers {
		for _, ref := range ps.Proto().Resource {
			if err := rp.checkAdd(ctx, pl, ref); err != nil {
				errs = append(errs, err)
			}
		}
	}

	if len(errs) > 0 {
		return nil, multierr.New(errs...)
	}

	platforms, err := planner.TargetPlatforms(ctx)
	if err != nil {
		return nil, err
	}

	var imageIDs []compute.Computable[oci.ImageID]
	var invocations []*resources.OpInvokeResourceProvider

	for _, resourceID := range rp.resourceIDs.Strings() {
		resource := rp.resources[resourceID]
		provider := resource.Provider

		if provider.InitializeWith.RequiresKeys || provider.InitializeWith.Snapshots != nil || provider.InitializeWith.Inject != nil {
			return nil, fnerrors.InternalError("bad resource provider initialization: unsupported inputs")
		}

		pkg, bin, err := pkggraph.LoadBinary(ctx, pl, provider.InitializeWith.BinaryRef)
		if err != nil {
			return nil, err
		}

		prepared, err := binary.PlanBinary(ctx, pl, env, pkg.Location, bin, binary.BuildImageOpts{
			UsePrebuilts: true,
			Platforms:    platforms,
		})
		if err != nil {
			return nil, err
		}

		imageID, _, err := ensureImage(ctx, pkggraph.MakeSealedContext(env, pl), prepared.Plan)
		if err != nil {
			return nil, err
		}

		imageIDs = append(imageIDs, imageID)

		invocations = append(invocations, &resources.OpInvokeResourceProvider{
			ResourceInstanceId: resource.ID,
			BinaryRef:          provider.InitializeWith.BinaryRef,
			BinaryConfig:       bin.Config,
			ResourceClass:      resource.Class.Spec,
			ResourceProvider:   provider,
			InstanceTypeSource: resource.Class.InstanceType.Sources,
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

type resourcePlanner struct {
	resourceIDs uniquestrings.List

	resources map[string]resourceInstance
}

type resourceInstance struct {
	ID       string
	Class    pkggraph.ResourceClass
	Provider *schema.ResourceProvider
}

func (rp *resourcePlanner) checkAdd(ctx context.Context, pl pkggraph.PackageLoader, resourceRef *schema.PackageRef) error {
	resourceID := fmt.Sprintf("%s:%s", resourceRef.AsPackageName(), resourceRef.Name)

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
	}

	rp.resources[instance.ID] = instance
	return nil
}
