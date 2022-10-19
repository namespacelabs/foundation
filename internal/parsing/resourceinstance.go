// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package parsing

import (
	"context"

	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnerrors/multierr"
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

func IsSecretResource(ref packageRefLike) bool {
	return ref.GetPackageName() == "namespacelabs.dev/foundation/std/runtime" && ref.GetName() == "Secret"
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

	name := &schema.PackageRef{PackageName: r.PackageName, Name: r.Name}
	ri := pkggraph.ResourceSpec{
		Source: r,
		Class:  *class,
	}

	loadedPrimitive, err := loadPrimitiveResources(ctx, pl, r)
	if err != nil {
		return nil, err
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
	} else if !loadedPrimitive {
		return nil, fnerrors.UserError(pp.Location, "missing provider for resource instance %q", r.Name)
	}

	if len(r.InputResource) > 0 {
		if r.Provider == "" {
			return nil, fnerrors.UserError(pp.Location, "input resources have been set, without a provider")
		}

		var resErrs []error
		for _, input := range r.InputResource {
			expected := ri.Provider.LookupExpected(input.Name)

			if expected == nil {
				resErrs = append(resErrs, fnerrors.BadInputError("resource %q is provided but not required", input.Name))
			} else {
				class := expected.Class
				resPkg, err := pl.LoadByName(ctx, input.ResourceRef.AsPackageName())
				if err != nil {
					resErrs = append(resErrs, fnerrors.BadInputError("resource %q failed to load package: %w", input.Name, err))
				} else {
					instance := resPkg.LookupResourceInstance(input.ResourceRef.Name)
					if instance == nil {
						resErrs = append(resErrs, fnerrors.BadInputError("resource %q refers to non-existing resource %q", input.Name, input.ResourceRef.Canonical()))
					} else if instance.Spec.Class.Ref.Canonical() != class.Ref.Canonical() {
						resErrs = append(resErrs, fnerrors.BadInputError("resource %q is of class %q, expected %q", input.Name, instance.Spec.Class.Ref.Canonical(), class.Ref.Canonical()))
					} else {
						ri.ResourceInputs = append(ri.ResourceInputs, pkggraph.ResourceInstance{
							Name: input.Name,
							Spec: instance.Spec,
						})
					}
				}
			}
		}

		if err := multierr.New(resErrs...); err != nil {
			return nil, err
		}
	}

	return &pkggraph.ResourceInstance{Name: name, Spec: ri}, nil
}

func loadPrimitiveResources(ctx context.Context, pl pkggraph.PackageLoader, r *schema.ResourceInstance) (bool, error) {
	// XXX Add generic package loading annotation to avoid special-casing this
	// resource class. Other type of resources could also have references to
	// packages.

	var pkg schema.PackageName
	switch {
	case IsServerResource(r.Class):
		intent := &runtime.ServerIntent{}
		if err := proto.Unmarshal(r.Intent.Value, intent); err != nil {
			return false, fnerrors.InternalError("failed to unwrap Server intent")
		}

		pkg = schema.PackageName(intent.PackageName)
	case IsSecretResource(r.Class):
		intent := &runtime.SecretIntent{}
		if err := proto.Unmarshal(r.Intent.Value, intent); err != nil {
			return false, fnerrors.InternalError("failed to unwrap Server intent")
		}

		pkg = intent.Ref.AsPackageName()
	}

	if pkg == "" {
		return false, nil
	}

	if _, err := pl.LoadByName(ctx, pkg); err != nil {
		return false, err
	}

	return true, nil
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
