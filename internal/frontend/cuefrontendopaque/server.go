// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cuefrontendopaque

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/protobuf/reflect/protoreflect"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/frontend/cuefrontend"
	"namespacelabs.dev/foundation/internal/frontend/cuefrontend/args"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

type cueServer struct {
	Name  string `json:"name"`
	Class string `json:"class"`

	Args *args.ArgsListOrMap `json:"args"`
	Env  *args.EnvMap        `json:"env"`

	Services map[string]cueService `json:"services"`

	Permissions *cuePermissions `json:"unstable_permissions,omitempty"`
}

type cuePermissions struct {
	ClusterRoles []string `json:"clusterRoles"`
}

// TODO: converge the relevant parts with parseCueContainer.
func parseCueServer(ctx context.Context, env *schema.Environment, pl parsing.EarlyPackageLoader, pkg *pkggraph.Package, v *fncue.CueV) (*schema.Server, *schema.StartupPlan, error) {
	loc := pkg.Location

	var bits cueServer
	if err := v.Val.Decode(&bits); err != nil {
		return nil, nil, err
	}

	out := &schema.Server{
		MainContainer: &schema.SidecarContainer{},
	}
	out.Name = bits.Name
	out.Framework = schema.Framework_OPAQUE
	out.RunByDefault = true

	switch bits.Class {
	case "stateless", "", string(schema.DeployableClass_STATELESS):
		out.DeployableClass = string(schema.DeployableClass_STATELESS)
	case "stateful", string(schema.DeployableClass_STATEFUL):
		out.DeployableClass = string(schema.DeployableClass_STATEFUL)
		out.IsStateful = true
	default:
		return nil, nil, fnerrors.NewWithLocation(loc, "%s: server class is not supported", bits.Class)
	}

	for name, svc := range bits.Services {
		parsed, endpointType, err := parseService(loc, name, svc)
		if err != nil {
			return nil, nil, err
		}

		if endpointType == schema.Endpoint_INTERNET_FACING {
			out.Ingress = append(out.Ingress, parsed)
		} else {
			out.Service = append(out.Service, parsed)
		}

		if endpointType != schema.Endpoint_INTERNET_FACING && len(svc.Ingress.Details.HttpRoutes) > 0 {
			return nil, nil, fnerrors.NewWithLocation(loc, "http routes are not supported for a private service %q", name)
		}
	}

	envVars, err := bits.Env.Parsed(loc.PackageName)
	if err != nil {
		return nil, nil, err
	}

	startupPlan := &schema.StartupPlan{
		Args: bits.Args.Parsed(),
		Env:  envVars,
	}

	if mounts := v.LookupPath("mounts"); mounts.Exists() {
		parsedMounts, volumes, err := cuefrontend.ParseMounts(ctx, pl, loc, mounts)
		if err != nil {
			return nil, nil, fnerrors.NewWithLocation(loc, "parsing volumes failed: %w", err)
		}

		out.Volume = append(out.Volume, volumes...)
		out.MainContainer.Mount = parsedMounts
	}

	if resources := v.LookupPath("resources"); resources.Exists() {
		resourceList, err := cuefrontend.ParseResourceList(resources)
		if err != nil {
			return nil, nil, fnerrors.NewWithLocation(loc, "parsing resources failed: %w", err)
		}

		pack, err := resourceList.ToPack(ctx, env, pl, pkg)
		if err != nil {
			return nil, nil, err
		}

		out.ResourcePack = pack
	}

	var availableServers schema.PackageList
	availableServers.Add(pkg.PackageName())

	if requires := v.LookupPath("requires"); requires.Exists() {
		declaredStack, err := parseRequires(ctx, pl, loc, requires)
		if err != nil {
			return nil, nil, err
		}

		if len(declaredStack) > 0 && out.ResourcePack == nil {
			out.ResourcePack = &schema.ResourcePack{}
		}

		availableServers.AddMultiple(declaredStack...)

		if err := parsing.AddServersAsResources(ctx, pl, schema.MakePackageRef(pkg.PackageName(), out.Name), declaredStack, out.ResourcePack); err != nil {
			return nil, nil, err
		}
	}

	for _, env := range startupPlan.Env {
		if env.FromServiceEndpoint != nil {
			dep := env.FromServiceEndpoint.ServerRef.AsPackageName()
			if !availableServers.Includes(dep) {
				// TODO reconcider if we want to implicitly add the dependency NSL-357
				return nil, nil, fnerrors.NewWithLocation(loc, "environment variable %s cannot be fulfilled: missing required server %s", env.Name, dep)
			}
		}
	}

	if bits.Permissions != nil {
		out.Permissions = &schema.ServerPermissions{}

		for _, clusterRole := range bits.Permissions.ClusterRoles {
			parsed, err := cuefrontend.ParseResourceRef(ctx, pl, pkg.Location, clusterRole)
			if err != nil {
				return nil, nil, err
			}

			out.Permissions.ClusterRole = append(out.Permissions.ClusterRole, parsed)

			if out.ResourcePack == nil {
				out.ResourcePack = &schema.ResourcePack{}
			}
			out.ResourcePack.ResourceRef = append(out.ResourcePack.ResourceRef, parsed)
		}
	}

	return out, startupPlan, nil
}

func validateStartupPlan(ctx context.Context, pl parsing.EarlyPackageLoader, pkg *pkggraph.Package, startupPlan *schema.StartupPlan) error {
	return validateEnvironment(ctx, pl, pkg, startupPlan.Env)
}

func validateEnvironment(ctx context.Context, pl parsing.EarlyPackageLoader, pkg *pkggraph.Package, env []*schema.BinaryConfig_EnvEntry) error {
	// Ensure each ref is loaded.
	for _, e := range env {
		switch {
		case e.FromSecretRef != nil:
			if _, err := ensureLoad(ctx, pl, pkg, e.FromSecretRef); err != nil {
				return err
			}

		case e.FromServiceEndpoint.GetServerRef() != nil:
			if _, err := ensureLoad(ctx, pl, pkg, e.FromServiceEndpoint.GetServerRef()); err != nil {
				return err
			}

		case e.FromResourceField.GetResource() != nil:
			targetPkg, err := ensureLoad(ctx, pl, pkg, e.FromResourceField.GetResource())
			if err != nil {
				return err
			}

			selector, err := canonicalizeFieldSelector(ctx, pl, pkg.Location, e.FromResourceField, targetPkg)
			if err != nil {
				return err
			}
			e.FromResourceField.FieldSelector = selector
		}
	}

	return nil
}

func canonicalizeFieldSelector(ctx context.Context, pl parsing.EarlyPackageLoader, loc pkggraph.Location, field *schema.ResourceConfigFieldSelector, targetPkg *pkggraph.Package) (string, error) {
	resource := field.GetResource()

	topLevelInstance := targetPkg.LookupResourceInstance(resource.Name)

	if topLevelInstance != nil {
		return canonicalizeClassInstanceFieldRef(ctx, pl, loc, topLevelInstance.Spec.Class.Ref, field.GetFieldSelector())
	} else {
		// Maybe it's an inline resource?
		for _, r := range targetPkg.Server.GetResourcePack().GetResourceInstance() {
			if r.Name == resource.Name {
				return canonicalizeClassInstanceFieldRef(ctx, pl, loc, r.Class, field.GetFieldSelector())
			}
		}

		return "", fnerrors.NewWithLocation(loc, "%s: no such resource", resource.Canonical())
	}

}

func canonicalizeClassInstanceFieldRef(ctx context.Context, pl parsing.EarlyPackageLoader, loc pkggraph.Location, classRef *schema.PackageRef, fieldSelector string) (string, error) {
	class, err := pkggraph.LookupResourceClass(ctx, pl, classRef)
	if err != nil {
		return "", err
	}

	return canonicalizeJsonPath(loc, class.InstanceType.Descriptor, class.InstanceType.Descriptor, fieldSelector, fieldSelector)
}

func canonicalizeJsonPath(loc pkggraph.Location, originalDesc, desc protoreflect.MessageDescriptor, originalSel, fieldSel string) (string, error) {
	parts := strings.SplitN(fieldSel, ".", 2)

	f := desc.Fields().ByTextName(parts[0])
	if f == nil {
		f = desc.Fields().ByJSONName(parts[0])
	}

	if f == nil {
		return "", fnerrors.NewWithLocation(loc, "%s: %q is not a valid field selector (%q doesn't match anything)", originalDesc.FullName(), originalSel, parts[0])
	}

	if len(parts) == 1 {
		if isSupportedProtoPrimitive(f) {
			return string(f.Name()), nil
		} else {
			return "", fnerrors.NewWithLocation(loc, "%s: %q is not a valid field selector (%q picks unsupported %v)", originalDesc.FullName(), originalSel, parts[0], f.Kind())
		}
	}

	if f.Kind() != protoreflect.MessageKind {
		var hint string
		if isSupportedProtoPrimitive(f) {
			hint = ": cannot select fields inside primitive types"
		}

		return "", fnerrors.NewWithLocation(loc, "%s: %q is not a valid field selector (%q picks unsupported %v)%s", originalDesc.FullName(), originalSel, parts[0], f.Kind(), hint)
	}

	selector, err := canonicalizeJsonPath(loc, originalDesc, f.Message(), originalSel, parts[1])
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s.%s", f.Name(), selector), nil
}

func isSupportedProtoPrimitive(f protoreflect.FieldDescriptor) bool {
	switch f.Kind() {
	case protoreflect.StringKind, protoreflect.Int32Kind, protoreflect.Uint32Kind, protoreflect.Int64Kind, protoreflect.Uint64Kind:
		return true

	default:
		return false
	}
}

func ensureLoad(ctx context.Context, pl parsing.EarlyPackageLoader, parent *pkggraph.Package, ref *schema.PackageRef) (*pkggraph.Package, error) {
	pkg := ref.AsPackageName()
	if pkg != parent.PackageName() {
		return pl.LoadByName(ctx, pkg)
	}

	return parent, nil
}
