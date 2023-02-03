// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cuefrontendopaque

import (
	"context"
	"fmt"

	"k8s.io/utils/strings/slices"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/frontend/cuefrontend"
	"namespacelabs.dev/foundation/internal/frontend/cuefrontend/args"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/internal/parsing/invariants"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/std/resources"
)

var (
	extensionFields = []string{
		"args", "env", "services", "unstable_permissions", "permissions", "probe", "probes", "security",
		"mounts", "resources", "requires", "tolerations",
		// This is needed for the "spec" in server templates. This can't be a private field, otherwise it can't be overridden.
		"spec"}

	serverFields = append(slices.Clone(extensionFields),
		"name", "class", "integration", "image", "imageFrom", "unstable_naming", "replicas", "spec")
)

type cueServer struct {
	Name  string `json:"name"`
	Class string `json:"class"`

	Replicas int32 `json:"replicas"`

	cueServerExtension
}

type cueServerExtension struct {
	Args *args.ArgsListOrMap `json:"args"`
	Env  *args.EnvMap        `json:"env"`

	Services map[string]cueService `json:"services"`

	UnstablePermissions *cuePermissions `json:"unstable_permissions,omitempty"`
	Permissions         *cuePermissions `json:"permissions,omitempty"`

	ReadinessProbe *cueProbe                   `json:"probe"`  // `probe: exec: "foo-cmd"`
	Probes         map[string]cueProbe         `json:"probes"` // `probes: readiness: exec: "foo-cmd"`
	Security       *cueServerSecurity          `json:"security,omitempty"`
	Tolerations    []*schema.Server_Toleration `json:"tolerations,omitempty"`

	Extensions []string `json:"extensions,omitempty"`
}

type cuePermissions struct {
	ClusterRoles []string `json:"clusterRoles"`
}

type cueServerSecurity struct {
	Privileged  bool `json:"privileged"`
	HostNetwork bool `json:"hostNetwork"`
}

// TODO: converge the relevant parts with parseCueContainer.
func parseCueServer(ctx context.Context, env *schema.Environment, pl parsing.EarlyPackageLoader, pkg *pkggraph.Package, v *fncue.CueV) (*schema.Server, error) {
	loc := pkg.Location

	if err := cuefrontend.ValidateNoExtraFields(loc, "server" /* messagePrefix */, v, serverFields); err != nil {
		return nil, err
	}

	var bits cueServer
	if err := v.Val.Decode(&bits); err != nil {
		return nil, err
	}

	fragment, err := parseServerExtension(ctx, env, pl, pkg, bits.cueServerExtension, v)
	if err != nil {
		return nil, err
	}

	out := &schema.Server{
		Name:         bits.Name,
		Framework:    schema.Framework_OPAQUE,
		RunByDefault: true,
		Replicas:     bits.Replicas,
		Self:         fragment,
	}

	switch bits.Class {
	case "stateless", "", string(schema.DeployableClass_STATELESS):
		out.DeployableClass = string(schema.DeployableClass_STATELESS)
	case "stateful", string(schema.DeployableClass_STATEFUL):
		out.DeployableClass = string(schema.DeployableClass_STATEFUL)
	case "daemonset", string(schema.DeployableClass_DAEMONSET):
		out.DeployableClass = string(schema.DeployableClass_DAEMONSET)
		if bits.Replicas > 0 {
			return nil, fnerrors.NewWithLocation(loc, "daemon set deployments do not support custom replica counts")
		}
	default:
		return nil, fnerrors.NewWithLocation(loc, "%s: server class is not supported", bits.Class)
	}

	return out, nil
}

func parseCueServerExtension(ctx context.Context, env *schema.Environment, pl parsing.EarlyPackageLoader, pkg *pkggraph.Package, v *fncue.CueV) (*schema.ServerFragment, error) {
	loc := pkg.Location

	if err := cuefrontend.ValidateNoExtraFields(loc, "extension" /* messagePrefix */, v, extensionFields); err != nil {
		return nil, err
	}

	var bits cueServerExtension
	if err := v.Val.Decode(&bits); err != nil {
		return nil, err
	}

	return parseServerExtension(ctx, env, pl, pkg, bits, v)
}

func parseServerExtension(ctx context.Context, env *schema.Environment, pl parsing.EarlyPackageLoader, pkg *pkggraph.Package, bits cueServerExtension, v *fncue.CueV) (*schema.ServerFragment, error) {
	loc := pkg.Location

	out := &schema.ServerFragment{
		MainContainer: &schema.Container{},
	}

	// Field validation needs to be done separately, because after parsing to JSON extra fields are lost.
	if services := v.LookupPath("services"); services.Exists() {
		it, err := services.Val.Fields()
		if err != nil {
			return nil, err
		}

		for it.Next() {
			if err := cuefrontend.ValidateNoExtraFields(loc, fmt.Sprintf("service %q:", it.Label()) /* messagePrefix */, &fncue.CueV{Val: it.Value()}, serviceFields); err != nil {
				return nil, err
			}
		}
	}

	var serviceProbes []*schema.Probe
	for name, svc := range bits.Services {
		parsed, probes, err := parseService(ctx, pl, loc, name, svc)
		if err != nil {
			return nil, err
		}

		if parsed.EndpointType == schema.Endpoint_INTERNET_FACING {
			out.Ingress = append(out.Ingress, parsed)
		} else {
			out.Service = append(out.Service, parsed)
		}

		if parsed.EndpointType != schema.Endpoint_INTERNET_FACING && len(svc.Ingress.Details.HttpRoutes) > 0 {
			return nil, fnerrors.NewWithLocation(loc, "http routes are not supported for a private service %q", name)
		}

		serviceProbes = append(serviceProbes, probes...)
	}

	var err error
	out.Probe, err = parseProbes(loc, serviceProbes, bits)
	if err != nil {
		return nil, err
	}

	out.MainContainer.Args = bits.Args.Parsed()
	out.MainContainer.Env, err = bits.Env.Parsed(ctx, pl, loc.PackageName)
	if err != nil {
		return nil, err
	}

	if sidecars := v.LookupPath("sidecars"); sidecars.Exists() {
		it, err := sidecars.Val.Fields()
		if err != nil {
			return nil, err
		}

		for it.Next() {
			val := &fncue.CueV{Val: it.Value()}

			if err := cuefrontend.ValidateNoExtraFields(loc, fmt.Sprintf("sidecar %q:", it.Label()) /* messagePrefix */, val, sidecarFields); err != nil {
				return nil, err
			}

			parsedContainer, err := parseCueContainer(ctx, env, pl, pkg, it.Label(), loc, val)
			if err != nil {
				return nil, err
			}

			out.Volume = append(out.Volume, parsedContainer.volumes...)
			pkg.Binaries = append(pkg.Binaries, parsedContainer.inlineBinaries...)

			if v, _ := val.LookupPath("init").Val.Bool(); v {
				out.InitContainer = append(out.InitContainer, parsedContainer.container)
			} else {
				out.Sidecar = append(out.Sidecar, parsedContainer.container)
			}
		}
	}

	if mounts := v.LookupPath("mounts"); mounts.Exists() {
		parsedMounts, volumes, err := cuefrontend.ParseMounts(ctx, pl, loc, mounts)
		if err != nil {
			return nil, fnerrors.NewWithLocation(loc, "parsing volumes failed: %w", err)
		}

		out.Volume = append(out.Volume, volumes...)
		out.MainContainer.Mount = parsedMounts
	}

	// XXX Hm, if anything this should be other way around.
	out.Volume = append(out.Volume, pkg.Volumes...)

	if resources := v.LookupPath("resources"); resources.Exists() {
		resourceList, err := cuefrontend.ParseResourceList(resources)
		if err != nil {
			return nil, fnerrors.NewWithLocation(loc, "parsing resources failed: %w", err)
		}

		pack, err := resourceList.ToPack(ctx, env, pl, pkg)
		if err != nil {
			return nil, err
		}

		out.ResourcePack = pack
	}

	var availableServers schema.PackageList
	availableServers.Add(pkg.PackageName())

	if requires := v.LookupPath("requires"); requires.Exists() {
		declaredStack, err := parseRequires(ctx, pl, loc, requires)
		if err != nil {
			return nil, err
		}

		availableServers.AddMultiple(declaredStack...)

		if err := parsing.AddServersAsResources(ctx, pl, schema.MakePackageSingleRef(pkg.PackageName()), declaredStack, out); err != nil {
			return nil, err
		}
	}

	for _, env := range out.MainContainer.Env {
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
			return nil, fnerrors.NewWithLocation(loc, "environment variable %s cannot be fulfilled:\nserver %q is referenced in %q but it is not in the server stack. Please explitly add this server to the `requires` list.\n", env.Name, dep, builtinName)
		}
	}

	permissions := bits.UnstablePermissions
	if bits.Permissions != nil {
		if err := parsing.RequireFeature(loc.Module, "experimental/kubernetes/permissions"); err != nil {
			return nil, fnerrors.AttachLocation(loc, err)
		}
		permissions = bits.Permissions
	}

	if permissions != nil {
		out.Permissions = &schema.ServerPermissions{}

		for _, clusterRole := range permissions.ClusterRoles {
			parsed, err := pkggraph.ParseAndLoadRef(ctx, pl, pkg.Location, clusterRole)
			if err != nil {
				return nil, err
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
			return nil, fnerrors.AttachLocation(loc, err)
		}

		out.MainContainer.Security = &schema.Container_Security{
			Privileged:  bits.Security.Privileged,
			HostNetwork: bits.Security.HostNetwork,
		}
	}

	if len(bits.Tolerations) > 0 {
		if err := parsing.RequireFeature(loc.Module, "experimental/container/tolerations"); err != nil {
			return nil, fnerrors.AttachLocation(loc, err)
		}

		out.Toleration = bits.Tolerations
	}

	for _, ext := range bits.Extensions {
		pkg := schema.PackageName(ext)
		if err := invariants.EnsurePackageLoaded(ctx, pl, loc.PackageName, pkg); err != nil {
			return nil, err
		}
		out.Extension = append(out.Extension, ext)
	}

	return out, nil
}

func ensureLoad(ctx context.Context, pl parsing.EarlyPackageLoader, parent *pkggraph.Package, ref *schema.PackageRef) (*pkggraph.Package, error) {
	pkg := ref.AsPackageName()
	if pkg != parent.PackageName() {
		return pl.LoadByName(ctx, pkg)
	}

	return parent, nil
}
