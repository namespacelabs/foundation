// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package parsing

import (
	"context"
	"encoding/json"
	"fmt"

	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/framework/rpcerrors/multierr"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/protos"
	"namespacelabs.dev/foundation/internal/support/naming"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

const Version_LibraryIntentsChanged = 48

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

func loadResourceInstance(ctx context.Context, pl pkggraph.PackageLoader, pkg *pkggraph.Package, instance *schema.ResourceInstance) (*pkggraph.ResourceInstance, error) {
	loc := pkg.Location

	if instance.IntentFrom != nil {
		if _, _, err := pkggraph.LoadBinary(ctx, pl, instance.IntentFrom.BinaryRef); err != nil {
			return nil, err
		}
	}

	class, err := pkggraph.LookupResourceClass(ctx, pl, instance.Class)
	if err != nil {
		return nil, err
	}

	name := &schema.PackageRef{PackageName: instance.PackageName, Name: instance.Name}
	ri := pkggraph.ResourceSpec{
		Source: protos.Clone(instance),
		Class:  *class,
	}

	provider := schema.PackageName(instance.Provider)
	if provider == "" {
		provider = class.DefaultProvider
	}

	if provider != "" {
		provider, err := LookupResourceProvider(ctx, pl, pkg, provider.String(), instance.Class)
		if err != nil {
			return nil, err
		}

		ri.Provider = provider
	}

	if ri.Provider != nil && ri.Provider.IntentType != nil {
		ri.IntentType = ri.Provider.IntentType
	} else if class.IntentType != nil {
		ri.IntentType = class.IntentType
	}

	if instance.SerializedIntentJson != "" {
		if ri.IntentType == nil {
			return nil, fnerrors.NewWithLocation(loc, "missing intent type for instance %q", instance.Name)
		}

		var raw any
		if err := json.Unmarshal([]byte(instance.SerializedIntentJson), &raw); err != nil {
			return nil, fnerrors.InternalError("failed to unmarshal serialized intent: %w", err)
		}

		resourceLoc, err := pl.Resolve(ctx, schema.PackageName(instance.PackageName))
		if err != nil {
			return nil, fnerrors.InternalError("failed to resolve %q: %w", instance.PackageName, err)
		}

		parsed, err := parseRawIntent(ctx, pl, pkg, resourceLoc, ri.IntentType, raw)
		if err != nil {
			return nil, fnerrors.NewWithLocation(loc, "failed to parse intent %q: %w", instance.Name, err)
		}

		ri.Intent = parsed

		if err := checkLoadPrimitiveResources(ctx, pl, loc.PackageName, ri.Class.Source, parsed); err != nil {
			return nil, err
		}
	}

	if len(instance.InputResource) > 0 {
		if ri.Provider == nil {
			return nil, fnerrors.NewWithLocation(loc, "input resources have been set, without a provider")
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

func parseRawIntent(ctx context.Context, pl pkggraph.PackageLoader, pkg *pkggraph.Package, loc pkggraph.Location, intentType *pkggraph.UserType, value any) (*anypb.Any, error) {
	subFsys := loc.Module.ReadOnlyFS(loc.Rel())

	msg, err := allocateWellKnownMessage(parseContext{
		FS:          subFsys,
		PackageName: loc.PackageName,
		EnsurePackage: func(requested schema.PackageName) error {
			if requested == pkg.PackageName() {
				return nil
			}
			_, err := pl.LoadByName(ctx, requested)
			return err
		},
	}, intentType.Descriptor, value)
	if err != nil {
		return nil, err
	}

	return anypb.New(msg)
}

func LookupResourceProvider(ctx context.Context, pl pkggraph.PackageLoader, pkg *pkggraph.Package, provider string, classRef *schema.PackageRef) (*pkggraph.ResourceProvider, error) {
	var providerPkg *pkggraph.Package

	// Is this a resource that references a provider in the same package?
	if provider == pkg.PackageName().String() {
		providerPkg = pkg
		if pkg == nil {
			return nil, fnerrors.InternalError("resource references %q as the provider but no provider package was specified", provider)
		}
	} else {
		loadedPkg, err := pl.LoadByName(ctx, schema.PackageName(provider))
		if err != nil {
			return nil, err
		}
		providerPkg = loadedPkg
	}

	p := providerPkg.LookupResourceProvider(classRef)
	if p == nil {
		return nil, fnerrors.NewWithLocation(pkg.Location, "package %q is not a provider for resource class %q", providerPkg.PackageName(), classRef.Canonical())
	}

	return p, nil
}

func checkLoadPrimitiveResources(ctx context.Context, pl pkggraph.PackageLoader, owner schema.PackageName, class *schema.ResourceClass, value *anypb.Any) error {
	if err := pkggraph.ValidateFoundation("runtime resources", Version_LibraryIntentsChanged, pkggraph.ModuleFromLoader(ctx, pl)); err != nil {
		return err
	}

	return nil
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

func AddServersAsResources(ctx context.Context, pl pkggraph.PackageLoader, owner *schema.PackageRef, servers []schema.PackageName, pack *schema.ResourcePack) error {
	for _, s := range servers {
		intent, err := json.Marshal(&schema.PackageRef{
			PackageName: s.String(),
		})
		if err != nil {
			return err
		}

		name := naming.StableIDN(fmt.Sprintf("%s->%s", owner.Canonical(), s.String()), 8)

		if _, err := pl.LoadByName(ctx, schema.PackageName(s.String())); err != nil {
			return err
		}

		pack.ResourceInstance = append(pack.ResourceInstance, &schema.ResourceInstance{
			PackageName:          owner.PackageName,
			Name:                 name,
			Class:                &schema.PackageRef{PackageName: "namespacelabs.dev/foundation/library/runtime", Name: "Server"},
			SerializedIntentJson: string(intent),
		})
	}

	if len(servers) > 0 {
		if _, err := pl.LoadByName(ctx, "namespacelabs.dev/foundation/library/runtime"); err != nil {
			return err
		}
	}

	return nil
}
