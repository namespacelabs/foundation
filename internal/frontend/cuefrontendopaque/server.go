// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cuefrontendopaque

import (
	"context"

	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/frontend/cuefrontend"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/workspace"
)

type cueServer struct {
	Name  string `json:"name"`
	Class string `json:"class"`

	Args *cuefrontend.ArgsListOrMap `json:"args"`
	Env  map[string]string          `json:"env"`

	Services  map[string]cueService `json:"services"`
	Resources []string              `json:"resources"`
}

type cueService struct {
	Kind    string     `json:"kind"`
	Port    int        `json:"port"`
	Ingress cueIngress `json:"ingress"`
}

type cueIngress struct {
	InternetFacing bool                `json:"internetFacing"`
	HttpRoutes     map[string][]string `json:"httpRoutes"`
}

// TODO: converge the relevant parts with parseCueContainer.
func parseCueServer(ctx context.Context, pl workspace.EarlyPackageLoader, loc pkggraph.Location, v *fncue.CueV) (*schema.Server, *schema.StartupPlan, error) {
	var bits cueServer
	if err := v.Val.Decode(&bits); err != nil {
		return nil, nil, err
	}

	out := &schema.Server{
		MainContainer: &schema.SidecarContainer{},
	}
	out.Name = bits.Name
	out.Framework = schema.Framework_BASE
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

	startupPlan := &schema.StartupPlan{
		Env:  bits.Env,
		Args: bits.Args.Parsed(),
	}

	if mounts := v.LookupPath("mounts"); mounts.Exists() {
		parsedMounts, inlinedVolumes, err := cuefrontend.ParseMounts(ctx, pl, loc, mounts)
		if err != nil {
			return nil, nil, fnerrors.Wrapf(loc, err, "parsing volumes")
		}

		out.Volumes = append(out.Volumes, inlinedVolumes...)
		out.MainContainer.Mounts = parsedMounts
	}

	for _, resource := range bits.Resources {
		r, err := parseResourceRef(ctx, pl, loc, resource)
		if err != nil {
			return nil, nil, err
		}

		out.Resources = append(out.Resources, r)
	}

	return out, startupPlan, nil
}

func parseService(loc pkggraph.Location, name string, svc cueService) (*schema.Server_ServiceSpec, schema.Endpoint_Type, error) {
	if svc.Kind != "http" && svc.Kind != "tcp" {
		return nil, schema.Endpoint_INGRESS_UNSPECIFIED, fnerrors.UserError(loc, "service kind is not supported: %s", svc.Kind)
	}

	var endpointType schema.Endpoint_Type
	if svc.Ingress.InternetFacing {
		endpointType = schema.Endpoint_INTERNET_FACING
	} else {
		endpointType = schema.Endpoint_PRIVATE
	}

	urlMap := &schema.HttpUrlMap{}
	for _, routes := range svc.Ingress.HttpRoutes {
		for _, route := range routes {
			urlMap.Entry = append(urlMap.Entry, &schema.HttpUrlMap_Entry{
				PathPrefix: route,
			})
		}
	}
	var details *anypb.Any
	if len(urlMap.Entry) > 0 {
		details = &anypb.Any{}
		err := details.MarshalFrom(urlMap)
		if err != nil {
			return nil, schema.Endpoint_INGRESS_UNSPECIFIED, err
		}
	}
	parsed := &schema.Server_ServiceSpec{
		Name: name,
		Port: &schema.Endpoint_Port{Name: name, ContainerPort: int32(svc.Port)},
		Metadata: []*schema.ServiceMetadata{{
			Protocol: svc.Kind,
			Details:  details,
		}},
	}

	return parsed, endpointType, nil
}
