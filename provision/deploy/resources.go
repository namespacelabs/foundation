// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package deploy

import (
	"context"
	"fmt"

	"namespacelabs.dev/foundation/build/binary"
	"namespacelabs.dev/foundation/engine/compute"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnerrors/multierr"
	"namespacelabs.dev/foundation/internal/support/naming"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/std/planning"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func PlanResources(ctx context.Context, pl pkggraph.SealedPackageLoader, env planning.Context, planner runtime.Planner, stack *provision.Stack) (*runtime.DeploymentPlan, error) {
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

	var spec runtime.DeploymentSpec
	var imageIDs []compute.Computable[oci.ImageID]
	for _, ref := range rp.resourceRefs.Refs() {
		resource := rp.resources[ref.Canonical()]
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

		spec.Specs = append(spec.Specs, runtime.DeployableSpec{
			ErrorLocation: resource.ProviderPackage.Location,

			PackageName: ref.AsPackageName(),
			Class:       schema.DeployableClass_ONESHOT,
			Id:          naming.StableID(fmt.Sprintf("%s-%s", provider.PackageName, provider.ProvidesClass.Canonical())),
			MainContainer: runtime.ContainerRunOpts{
				Command:    prepared.Command,
				Args:       provider.InitializeWith.Args,
				WorkingDir: provider.InitializeWith.WorkingDir,
			},
		})
	}

	builtImages, err := compute.GetValue(ctx, compute.Collect(tasks.Action("resource-provider.build-image"), imageIDs...))
	if err != nil {
		return nil, err
	}

	for k, img := range builtImages {
		spec.Specs[k].MainContainer.Image = img.Value
	}

	return planner.PlanDeployment(ctx, spec)
}

type resourcePlanner struct {
	resourceRefs schema.PackageRefList

	resources map[string]pkggraph.Resource
}

func (rp *resourcePlanner) checkAdd(ctx context.Context, pl pkggraph.PackageLoader, resourceRef *schema.PackageRef) error {
	if !rp.resourceRefs.Add(resourceRef) {
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
		rp.resources = map[string]pkggraph.Resource{}
	}

	// XXX add resources recursively.

	rp.resources[resourceRef.Canonical()] = *resource
	return nil
}
