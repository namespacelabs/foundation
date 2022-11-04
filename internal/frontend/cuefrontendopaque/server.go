// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cuefrontendopaque

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/framework/rpcerrors/multierr"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/frontend/cuefrontend"
	"namespacelabs.dev/foundation/internal/frontend/cuefrontend/args"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/library/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

type cueServer struct {
	Name  string `json:"name"`
	Class string `json:"class"`

	Args *args.ArgsListOrMap `json:"args"`
	Env  *args.EnvMap        `json:"env"`

	Services map[string]cueService `json:"services"`
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
		return nil, nil, fnerrors.UserError(loc, "%s: server class is not supported", bits.Class)
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

		if endpointType != schema.Endpoint_INTERNET_FACING && len(svc.Ingress.HttpRoutes) > 0 {
			return nil, nil, fnerrors.UserError(loc, "http routes are not supported for a private service %q", name)
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
			return nil, nil, fnerrors.Wrapf(loc, err, "parsing volumes")
		}

		out.Volume = append(out.Volume, volumes...)
		out.MainContainer.Mount = parsedMounts
	}

	if resources := v.LookupPath("resources"); resources.Exists() {
		resourceList, err := cuefrontend.ParseResourceList(resources)
		if err != nil {
			return nil, nil, fnerrors.Wrapf(loc, err, "parsing resources")
		}

		pack, err := resourceList.ToPack(ctx, env, pl, pkg)
		if err != nil {
			return nil, nil, err
		}

		out.ResourcePack = pack
	}

	if requires := v.LookupPath("requires"); requires.Exists() {
		declaredStack, err := parseRequires(ctx, pl, loc, requires)
		if err != nil {
			return nil, nil, err
		}

		if len(declaredStack) > 0 && out.ResourcePack == nil {
			out.ResourcePack = &schema.ResourcePack{}
		}

		var errs []error
		for k, server := range declaredStack {
			if _, err := pl.LoadByName(ctx, "namespacelabs.dev/foundation/library/runtime"); err != nil {
				errs = append(errs, err)
				continue
			}

			intent, err := anypb.New(&runtime.ServerIntent{
				PackageName: server.String(),
			})
			if err != nil {
				errs = append(errs, err)
				continue
			}

			out.ResourcePack.ResourceInstance = append(out.ResourcePack.ResourceInstance, &schema.ResourceInstance{
				PackageName: pkg.Location.PackageName.String(),
				Name:        fmt.Sprintf("$required_%d", k),
				Class:       &schema.PackageRef{PackageName: "namespacelabs.dev/foundation/library/runtime", Name: "Server"},
				Intent:      intent,
			})
		}

		if err := multierr.New(errs...); err != nil {
			return nil, nil, err
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
			resource := e.FromResourceField.GetResource()
			targetPkg, err := ensureLoad(ctx, pl, pkg, resource)
			if err != nil {
				return err
			}

			topLevelInstance := targetPkg.LookupResourceInstance(resource.Name)
			if topLevelInstance != nil {
				return validateClassInstanceFieldRef(ctx, pl, topLevelInstance.Spec.Class.Ref, e.FromResourceField.FieldSelector)
			} else {
				// Maybe it's an inline resource?
				for _, r := range targetPkg.Server.GetResourcePack().GetResourceInstance() {
					if r.Name == resource.Name {
						return validateClassInstanceFieldRef(ctx, pl, r.Class, e.FromResourceField.GetFieldSelector())
					}
				}

				return fnerrors.BadInputError("%s: no such resource", resource.Canonical())
			}
		}
	}

	return nil
}

func validateClassInstanceFieldRef(ctx context.Context, pl parsing.EarlyPackageLoader, classRef *schema.PackageRef, fieldSelector string) error {
	class, err := pkggraph.LookupResourceClass(ctx, pl, classRef)
	if err != nil {
		return err
	}

	return validateJsonPath(class.InstanceType.Descriptor, class.InstanceType.Descriptor, fieldSelector, fieldSelector)
}

func validateJsonPath(originalDesc, desc protoreflect.MessageDescriptor, originalSel, fieldSel string) error {
	parts := strings.SplitN(fieldSel, ".", 2)

	f := desc.Fields().ByJSONName(parts[0])
	if f == nil {
		return fnerrors.BadInputError("%s: %q is not a valid field selector (%q doesn't match anything)", originalDesc.FullName(), originalSel, parts[0])
	}

	if len(parts) == 1 {
		switch f.Kind() {
		case protoreflect.StringKind, protoreflect.Int32Kind, protoreflect.Uint32Kind, protoreflect.Int64Kind, protoreflect.Uint64Kind:
			return nil

		default:
			return fnerrors.BadInputError("%s: %q is not a valid field selector (%q picks unsupported %v)", originalDesc.FullName(), originalSel, parts[0], f.Kind())
		}
	}

	if f.Kind() != protoreflect.MessageKind {
		return fnerrors.BadInputError("%s: %q is not a valid field selector (%q picks unsupported %v)", originalDesc.FullName(), originalSel, parts[0], f.Kind())
	}

	return validateJsonPath(originalDesc, f.Message(), originalSel, parts[1])
}

func ensureLoad(ctx context.Context, pl parsing.EarlyPackageLoader, parent *pkggraph.Package, ref *schema.PackageRef) (*pkggraph.Package, error) {
	pkg := ref.AsPackageName()
	if pkg != parent.PackageName() {
		return pl.LoadByName(ctx, pkg)
	}

	return parent, nil
}
