// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cuefrontendopaque

import (
	"context"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/frontend/cuefrontend"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

type cueServer struct {
	Name  string `json:"name"`
	Class string `json:"class"`

	Args *cuefrontend.ArgsListOrMap `json:"args"`
	Env  *cuefrontend.EnvMap        `json:"env"`

	Services  map[string]cueService     `json:"services"`
	Resources *cuefrontend.ResourceList `json:"resources"`
}

// TODO: converge the relevant parts with parseCueContainer.
func parseCueServer(ctx context.Context, pl parsing.EarlyPackageLoader, loc pkggraph.Location, v *fncue.CueV) (*schema.Server, *schema.StartupPlan, error) {
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

	env, err := bits.Env.Parsed(loc.PackageName)
	if err != nil {
		return nil, nil, err
	}

	startupPlan := &schema.StartupPlan{
		Args: bits.Args.Parsed(),
		Env:  env,
	}

	if mounts := v.LookupPath("mounts"); mounts.Exists() {
		parsedMounts, inlinedVolumes, err := cuefrontend.ParseMounts(ctx, pl, loc, mounts)
		if err != nil {
			return nil, nil, fnerrors.Wrapf(loc, err, "parsing volumes")
		}

		out.Volume = append(out.Volume, inlinedVolumes...)
		out.MainContainer.Mount = parsedMounts
	}

	if bits.Resources != nil {
		pack, err := bits.Resources.ToPack(ctx, pl, loc)
		if err != nil {
			return nil, nil, err
		}

		out.ResourcePack = pack
	}

	return out, startupPlan, nil
}
