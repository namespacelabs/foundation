// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package parsing

import (
	"context"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/framework/rpcerrors/multierr"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/protos"
	"namespacelabs.dev/foundation/library/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

type packageRefLike interface {
	GetPackageName() string
	GetName() string
}

func isRuntimeResource(ref packageRefLike) bool {
	return ref.GetPackageName() == "namespacelabs.dev/foundation/library/runtime" || ref.GetPackageName() == "library.namespace.so/runtime"
}

func IsServerResource(ref packageRefLike) bool {
	return isRuntimeResource(ref) && ref.GetName() == "Server"
}

func IsSecretResource(ref packageRefLike) bool {
	return isRuntimeResource(ref) && ref.GetName() == "Secret"
}

func loadResourceInstance(ctx context.Context, pl pkggraph.PackageLoader, pp *pkggraph.Package, instance *schema.ResourceInstance) (*pkggraph.ResourceInstance, error) {
	instance.PackageName = string(pp.PackageName())

	if instance.Intent != nil && instance.IntentFrom != nil {
		return nil, fnerrors.UserError(pp.Location, "resource instance %q cannot specify both \"intent\" and \"from\"", instance.Name)
	}

	if instance.IntentFrom != nil {
		if _, _, err := pkggraph.LoadBinary(ctx, pl, instance.IntentFrom.BinaryRef); err != nil {
			return nil, err
		}
	}

	classPkg, err := pl.LoadByName(ctx, instance.Class.AsPackageName())
	if err != nil {
		return nil, err
	}

	class := classPkg.LookupResourceClass(instance.Class.Name)
	if class == nil {
		return nil, fnerrors.UserError(pp.Location, "no such resource class %q", instance.Class.Canonical())
	}

	name := &schema.PackageRef{PackageName: instance.PackageName, Name: instance.Name}
	ri := pkggraph.ResourceSpec{
		Source: protos.Clone(instance),
		Class:  *class,
	}

	loadedPrimitive, err := loadPrimitiveResources(ctx, pl, pp.Location.PackageName, instance)
	if err != nil {
		return nil, err
	}

	if instance.Provider != "" {
		providerPkg, err := pl.LoadByName(ctx, schema.PackageName(instance.Provider))
		if err != nil {
			return nil, err
		}

		provider := providerPkg.LookupResourceProvider(instance.Class)
		if provider == nil {
			return nil, fnerrors.UserError(pp.Location, "package %q does not a provider for resource class %q", instance.Provider, instance.Class.Canonical())
		}

		ri.Provider = provider
	} else if loadedPrimitive == nil {
		return nil, fnerrors.UserError(pp.Location, "missing provider for resource instance %q", instance.Name)
	} else {
		serialized, err := anypb.New(loadedPrimitive)
		if err != nil {
			return nil, fnerrors.InternalError("failed to re-serialize intent: %w", err)
		}
		ri.Source.Intent = serialized
	}

	if len(instance.InputResource) > 0 {
		if instance.Provider == "" {
			return nil, fnerrors.UserError(pp.Location, "input resources have been set, without a provider")
		}

		var resErrs []error
		for _, input := range instance.InputResource {
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
							ResourceRef: input.Name,
							Spec:        instance.Spec,
						})
					}
				}
			}
		}

		if err := multierr.New(resErrs...); err != nil {
			return nil, err
		}
	}

	return &pkggraph.ResourceInstance{ResourceRef: name, Spec: ri}, nil
}

func loadPrimitiveResources(ctx context.Context, pl pkggraph.PackageLoader, owner schema.PackageName, instance *schema.ResourceInstance) (proto.Message, error) {
	// XXX Add generic package loading annotation to avoid special-casing this
	// resource class. Other type of resources could also have references to
	// packages.

	var pkg schema.PackageName
	var msg proto.Message

	switch {
	case IsServerResource(instance.Class):
		intent := &runtime.ServerIntent{}
		if err := proto.Unmarshal(instance.Intent.Value, intent); err != nil {
			return nil, fnerrors.InternalError("failed to unwrap Server intent")
		}

		pkg = schema.PackageName(intent.PackageName)
		msg = intent

	case IsSecretResource(instance.Class):
		intent := &runtime.SecretIntent{}
		if err := proto.Unmarshal(instance.Intent.Value, intent); err != nil {
			return nil, fnerrors.InternalError("failed to unwrap Secret intent")
		}

		owner := schema.PackageName(instance.PackageName)
		ref, err := schema.ParsePackageRef(owner, intent.Ref)
		if err != nil {
			return nil, err
		}

		pkg = ref.AsPackageName()
		msg = &runtime.SecretIntent{
			Ref: ref.Canonical(),
		}
	}

	if pkg == "" {
		return nil, nil
	}

	if pkg != owner {
		if _, err := pl.LoadByName(ctx, pkg); err != nil {
			return nil, err
		}
	}

	return msg, nil
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
