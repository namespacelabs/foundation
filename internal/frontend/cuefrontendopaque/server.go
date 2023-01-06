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
	"namespacelabs.dev/foundation/std/resources"
)

var serverFields = []string{
	"name", "class", "args", "env", "services", "unstable_permissions", "permissions", "probe", "probes", "security",
	"mounts", "resources", "integration", "image", "imageFrom", "unstable_naming", "requires", "replicas",
	// This is needed for the "spec" in server templates. This can't be a private field, otherwise it can't be overridden.
	"spec",
}

type cueServer struct {
	Name  string `json:"name"`
	Class string `json:"class"`

	Replicas int32 `json:"replicas"`

	Args *args.ArgsListOrMap `json:"args"`
	Env  *args.EnvMap        `json:"env"`

	Services map[string]cueService `json:"services"`

	UnstablePermissions *cuePermissions `json:"unstable_permissions,omitempty"`
	Permissions         *cuePermissions `json:"permissions,omitempty"`

	ReadinessProbe *cueProbe           `json:"probe"`  // `probe: exec: "foo-cmd"`
	Probes         map[string]cueProbe `json:"probes"` // `probes: readiness: exec: "foo-cmd"`
	Security       *cueServerSecurity  `json:"security,omitempty"`
}

type cuePermissions struct {
	ClusterRoles []string `json:"clusterRoles"`
}

type cueServerSecurity struct {
	Privileged bool `json:"privileged"`
}

// TODO: converge the relevant parts with parseCueContainer.
func parseCueServer(ctx context.Context, env *schema.Environment, pl parsing.EarlyPackageLoader, pkg *pkggraph.Package, v *fncue.CueV) (*schema.Server, *schema.StartupPlan, error) {
	loc := pkg.Location

	if err := cuefrontend.ValidateNoExtraFields(loc, "server" /* messagePrefix */, v, serverFields); err != nil {
		return nil, nil, err
	}

	var bits cueServer
	if err := v.Val.Decode(&bits); err != nil {
		return nil, nil, err
	}

	out := &schema.Server{
		MainContainer: &schema.Container{},
		Name:          bits.Name,
		Framework:     schema.Framework_OPAQUE,
		RunByDefault:  true,
		Replicas:      bits.Replicas,
	}

	switch bits.Class {
	case "stateless", "", string(schema.DeployableClass_STATELESS):
		out.DeployableClass = string(schema.DeployableClass_STATELESS)
	case "stateful", string(schema.DeployableClass_STATEFUL):
		out.DeployableClass = string(schema.DeployableClass_STATEFUL)
	case "daemonset", string(schema.DeployableClass_DAEMONSET):
		out.DeployableClass = string(schema.DeployableClass_DAEMONSET)
		if bits.Replicas > 0 {
			return nil, nil, fnerrors.NewWithLocation(loc, "daemon set deployments do not support custom replica counts")
		}
	default:
		return nil, nil, fnerrors.NewWithLocation(loc, "%s: server class is not supported", bits.Class)
	}

	// Field validation needs to be done separately, because after parsing to JSON extra fields are lost.
	if services := v.LookupPath("services"); services.Exists() {
		it, err := services.Val.Fields()
		if err != nil {
			return nil, nil, err
		}

		for it.Next() {
			if err := cuefrontend.ValidateNoExtraFields(loc, fmt.Sprintf("service %q:", it.Label()) /* messagePrefix */, &fncue.CueV{Val: it.Value()}, serviceFields); err != nil {
				return nil, nil, err
			}
		}
	}

	var serviceProbes []*schema.Probe
	for name, svc := range bits.Services {
		parsed, probes, err := parseService(ctx, pl, loc, name, svc)
		if err != nil {
			return nil, nil, err
		}

		if parsed.EndpointType == schema.Endpoint_INTERNET_FACING {
			out.Ingress = append(out.Ingress, parsed)
		} else {
			out.Service = append(out.Service, parsed)
		}

		if parsed.EndpointType != schema.Endpoint_INTERNET_FACING && len(svc.Ingress.Details.HttpRoutes) > 0 {
			return nil, nil, fnerrors.NewWithLocation(loc, "http routes are not supported for a private service %q", name)
		}

		serviceProbes = append(serviceProbes, probes...)
	}

	var err error
	out.Probe, err = parseProbes(loc, serviceProbes, bits)
	if err != nil {
		return nil, nil, err
	}

	startupPlan := &schema.StartupPlan{
		Args: bits.Args.Parsed(),
	}

	startupPlan.Env, err = bits.Env.Parsed(loc.PackageName)
	if err != nil {
		return nil, nil, err
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
		var dep schema.PackageName
		var builtinName string
		if env.FromServiceEndpoint != nil {
			dep = env.FromServiceEndpoint.ServerRef.AsPackageName()
			builtinName = `fromServiceEndpoint`
		}
		if env.FromServiceIngress != nil {
			dep = env.FromServiceIngress.ServerRef.AsPackageName()
			builtinName = `fromServiceIngress`
		}

		if builtinName != "" && !availableServers.Has(dep) {
			// TODO reconcider if we want to implicitly add the dependency NSL-357
			return nil, nil, fnerrors.NewWithLocation(loc, "environment variable %s cannot be fulfilled:\nserver %q is referenced in %q but it is not in the server stack. Please explitly add this server to the `requires` list.\n", env.Name, dep, builtinName)
		}
	}

	permissions := bits.UnstablePermissions
	if bits.Permissions != nil {
		if err := parsing.RequireFeature(loc.Module, "experimental/kubernetes/permissions"); err != nil {
			return nil, nil, fnerrors.AttachLocation(loc, err)
		}
		permissions = bits.Permissions
	}

	if permissions != nil {
		out.Permissions = &schema.ServerPermissions{}

		for _, clusterRole := range permissions.ClusterRoles {
			parsed, err := pkggraph.ParseAndLoadRef(ctx, pl, pkg.Location, clusterRole)
			if err != nil {
				return nil, nil, err
			}

			out.Permissions.ClusterRole = append(out.Permissions.ClusterRole, &schema.ServerPermissions_ClusterRole{
				Label:      parsed.Canonical(),
				ResourceId: resources.ResourceID(parsed),
			})

			if out.ResourcePack == nil {
				out.ResourcePack = &schema.ResourcePack{}
			}
			out.ResourcePack.ResourceRef = append(out.ResourcePack.ResourceRef, parsed)
		}
	}

	if bits.Security != nil {
		if err := parsing.RequireFeature(loc.Module, "experimental/container/security"); err != nil {
			return nil, nil, fnerrors.AttachLocation(loc, err)
		}

		out.MainContainer.Security = &schema.Container_Security{
			Privileged: bits.Security.Privileged,
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

		case e.FromServiceIngress.GetServerRef() != nil:
			if _, err := ensureLoad(ctx, pl, pkg, e.FromServiceIngress.GetServerRef()); err != nil {
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
		// XXX rethink how scoped resource instances should work.

		for _, pack := range []*schema.ResourcePack{targetPkg.Extension.GetResourcePack(), targetPkg.Service.GetResourcePack(), targetPkg.Server.GetResourcePack()} {
			for _, r := range pack.GetResourceInstance() {
				if r.Name == resource.Name && r.PackageName == resource.PackageName {
					return canonicalizeClassInstanceFieldRef(ctx, pl, loc, r.Class, field.GetFieldSelector())
				}
			}
		}

		return "", fnerrors.NewWithLocation(loc, "%s: no such resource", resource.Canonical())
	}

}

func canonicalizeClassInstanceFieldRef(ctx context.Context, pl parsing.EarlyPackageLoader, loc pkggraph.Location, classRef *schema.PackageRef, fieldSelector string) (string, error) {
	class, err := pkggraph.LookupResourceClass(ctx, pl, nil, classRef)
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
