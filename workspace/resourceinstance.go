// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package workspace

import (
	"context"

	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/std/runtime"
)

type packageRefLike interface {
	GetPackageName() string
	GetName() string
}

func IsServerResource(ref packageRefLike) bool {
	return ref.GetPackageName() == "namespacelabs.dev/foundation/std/runtime" && ref.GetName() == "Server"
}

func loadResourceInstance(ctx context.Context, pl pkggraph.PackageLoader, pp *pkggraph.Package, r *schema.ResourceInstance) (*pkggraph.ResourceInstance, error) {
	r.PackageName = string(pp.PackageName())

	if r.Intent != nil && r.IntentFrom != nil {
		return nil, fnerrors.UserError(pp.Location, "resource instance %q cannot specify both \"intent\" and \"from\"", r.Name)
	}

	if r.IntentFrom != nil {
		if _, _, err := pkggraph.LoadBinary(ctx, pl, r.IntentFrom.BinaryRef); err != nil {
			return nil, err
		}
	}

	classPkg, err := pl.LoadByName(ctx, r.Class.AsPackageName())
	if err != nil {
		return nil, err
	}

	class := classPkg.LookupResourceClass(r.Class.Name)
	if class == nil {
		return nil, fnerrors.UserError(pp.Location, "no such resource class %q", r.Class.Canonical())
	}

	ri := &pkggraph.ResourceInstance{
		Ref:   &schema.PackageRef{PackageName: r.PackageName, Name: r.Name},
		Spec:  r,
		Class: *class,
	}

	// XXX Add generic package loading annotation to avoid special-casing this
	// resource class. Other type of resources could also have references to
	// packages.
	if IsServerResource(r.Class) {
		serverIntent := &runtime.ServerIntent{}
		if err := proto.Unmarshal(r.Intent.Value, serverIntent); err != nil {
			return nil, fnerrors.InternalError("failed to unwrap Server intent")
		}

		// Make sure that servers we refer to are package loaded.
		if _, err := pl.LoadByName(ctx, schema.PackageName(serverIntent.PackageName)); err != nil {
			return nil, err
		}
	}

	if r.Provider != "" {
		providerPkg, err := pl.LoadByName(ctx, schema.PackageName(r.Provider))
		if err != nil {
			return nil, err
		}

		provider := providerPkg.LookupResourceProvider(r.Class)
		if provider == nil {
			return nil, fnerrors.UserError(pp.Location, "package %q does not a provider for resource class %q", r.Provider, r.Class.Canonical())
		}

		ri.Provider = *provider
	} else if !IsServerResource(r.Class) {
		return nil, fnerrors.UserError(pp.Location, "missing provider for resource instance %q", r.Name)
	}

	return ri, nil
}

func LoadResources(ctx context.Context, pl pkggraph.PackageLoader, pkg *pkggraph.Package, pack *schema.ResourcePack) ([]pkggraph.ResourceInstance, error) {
	var resources []pkggraph.ResourceInstance

	for _, resource := range pack.GetResourceRef() {
		pkg, err := pl.LoadByName(ctx, resource.AsPackageName())
		if err != nil {
			return nil, err
		}

		res := pkg.LookupResourceInstance(resource.Name)
		if res == nil {
			return nil, fnerrors.BadInputError("%s: no such resource", resource.Canonical())
		}

		resources = append(resources, *res)
	}

	for _, resource := range pack.GetResourceInstance() {
		instance, err := loadResourceInstance(ctx, pl, pkg, resource)
		if err != nil {
			return nil, err
		}

		resources = append(resources, *instance)
	}

	return resources, nil
}
