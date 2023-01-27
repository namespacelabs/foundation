// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package parsing

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"golang.org/x/exp/maps"
	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/framework/kubernetes/kubenaming"
	"namespacelabs.dev/foundation/framework/rpcerrors/multierr"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/parsing/invariants"
	"namespacelabs.dev/foundation/internal/protos"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/std/resources"
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

func loadResourceInstance(ctx context.Context, pl pkggraph.PackageLoader, pkg *pkggraph.Package, parentID string, instance *schema.ResourceInstance) (*pkggraph.ResourceInstance, error) {
	loc := pkg.Location

	if instance.IntentFrom != nil {
		if err := pl.Ensure(ctx, instance.IntentFrom.BinaryRef.AsPackageName()); err != nil {
			return nil, err
		}
	}

	class, err := pkggraph.LookupResourceClass(ctx, pl, pkg, instance.Class)
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
			return nil, fnerrors.AttachLocation(name, err)
		}

		ri.Provider = provider
	}

	if ri.Provider == nil {
		return nil, fnerrors.NewWithLocation(loc, "missing provider for instance %q", instance.Name)
	}

	if ri.Provider.IntentType == nil {
		return nil, fnerrors.NewWithLocation(loc, "missing intent type for instance %q", instance.Name)
	}

	ri.IntentType = ri.Provider.IntentType

	if instance.SerializedIntentJson != "" {
		var raw any
		if err := NewJsonNumberDecoder(instance.SerializedIntentJson).Decode(&raw); err != nil {
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

	index := map[string]*schema.ResourceInstance_InputResource{}

	for _, input := range instance.InputResource {
		index[input.Name.Canonical()] = input
	}

	var errs []error

	if ri.Provider != nil {
		for _, expected := range ri.Provider.ResourceInputs {
			value, ok := index[expected.Name.Canonical()]
			var resourceRef *schema.PackageRef
			if !ok {
				if expected.DefaultResource != nil {
					resourceRef = expected.DefaultResource
				} else {
					errs = append(errs, fnerrors.New("resource input for %q is missing", expected.Name.Canonical()))
					continue
				}
			} else {
				resourceRef = value.ResourceRef
				delete(index, expected.Name.Canonical())
			}

			name := expected.Name.Canonical()
			var instance *pkggraph.ResourceInstance

			// Special handling for resources in the same package to avoid deadlocking the package loader.
			if resourceRef.AsPackageName() == pkg.PackageName() {
				var ri *schema.ResourceInstance
				for _, r := range pkg.ResourceInstanceSpecs {
					if r.Name == resourceRef.Name {
						ri = r
						break
					}

				}

				instance, err = loadResourceInstance(ctx, pl, pkg, parentID, ri)
				if err != nil {
					errs = append(errs, fnerrors.BadInputError("resource %q failed to load: %w", name, err))
					continue
				}
			} else {
				resPkg, err := pl.LoadByName(ctx, resourceRef.AsPackageName())
				if err != nil {
					errs = append(errs, fnerrors.BadInputError("resource %q failed to load package: %w", name, err))
					continue
				}

				instance = resPkg.LookupResourceInstance(resourceRef.Name)
			}

			if instance == nil {
				errs = append(errs, fnerrors.BadInputError("resource %q refers to non-existing resource %q", name, resourceRef.Canonical()))
			} else if instance.Spec.Class.Ref.Canonical() != expected.Class.Ref.Canonical() {
				errs = append(errs, fnerrors.BadInputError("resource %q is of class %q, expected %q", name,
					instance.Spec.Class.Ref.Canonical(), expected.Class.Ref.Canonical()))
			} else {
				ri.ResourceInputs = append(ri.ResourceInputs, pkggraph.ResourceInstance{
					ResourceID:  instance.ResourceID,
					ResourceRef: expected.Name,
					Spec:        instance.Spec,
				})
			}
		}
	}

	if len(index) > 0 {
		errs = append(errs, fnerrors.New("the following specified resource values are not required: %s", strings.Join(maps.Keys(index), ", ")))
	}

	if err := multierr.New(errs...); err != nil {
		return nil, err
	}

	return &pkggraph.ResourceInstance{
		ResourceID:  resources.ScopedID(parentID, name),
		ResourceRef: name,
		Spec:        ri,
	}, nil
}

func parseRawIntent(ctx context.Context, pl pkggraph.PackageLoader, pkg *pkggraph.Package, loc pkggraph.Location, intentType *pkggraph.UserType, value any) (*anypb.Any, error) {
	subFsys := loc.Module.ReadOnlyFS(loc.Rel())

	msg, err := protos.AllocateWellKnownMessage(protos.ParseContext{
		SupportWellKnownMessages: true,
		FS:                       subFsys,
		PackageName:              loc.PackageName,
		EnsurePackage: func(requested schema.PackageName) error {
			return invariants.EnsurePackageLoaded(ctx, pl, pkg.PackageName(), requested)
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
		return nil, fnerrors.New("package %q is not a provider for resource class %q", providerPkg.PackageName(), classRef.Canonical())
	}

	return p, nil
}

func checkLoadPrimitiveResources(ctx context.Context, pl pkggraph.PackageLoader, owner schema.PackageName, class *schema.ResourceClass, value *anypb.Any) error {
	if err := pkggraph.ValidateFoundation("runtime resources", Version_LibraryIntentsChanged, pkggraph.ModuleFromLoader(ctx, pl)); err != nil {
		return err
	}

	return nil
}

func LoadResources(ctx context.Context, pl pkggraph.PackageLoader, pkg *pkggraph.Package, parentID string, pack *schema.ResourcePack) ([]pkggraph.ResourceInstance, error) {
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
		instance, err := loadResourceInstance(ctx, pl, pkg, parentID, resource)
		if err != nil {
			return nil, err
		}

		resources = append(resources, *instance)
	}

	return resources, nil
}

func AddServersAsResources(ctx context.Context, pl pkggraph.PackageLoader, owner *schema.PackageRef, servers []schema.PackageName, pack *schema.ResourcePack) error {
	if owner.PackageName == "" {
		return fnerrors.InternalError("owner.package_name is missing")
	}

	for _, s := range servers {
		intent, err := json.Marshal(&schema.PackageRef{
			PackageName: s.String(),
		})
		if err != nil {
			return err
		}

		name := kubenaming.StableIDN(fmt.Sprintf("%s->%s", owner.Canonical(), s.String()), 8)

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
