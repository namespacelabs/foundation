// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cuefrontend

import (
	"context"
	"sort"
	"strings"

	"cuelang.org/go/cue"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
)

type cueServer struct {
	ID         string                    `json:"id"`
	Name       string                    `json:"name"`
	Framework  string                    `json:"framework"`
	IsStateful bool                      `json:"isStateful"`
	Import     []string                  `json:"import"`
	Services   map[string]cueServiceSpec `json:"service"`
	StaticEnv  map[string]string         `json:"env"`
	Binary     cueServerBinary           `json:"binary"`

	// XXX this should be somewhere else.
	URLMap []cueURLMapEntry `json:"urlmap"`
}

type cueURLMapEntry struct {
	Path   string             `json:"path"`
	Import schema.PackageName `json:"import"`
}

type cueServiceSpec struct {
	Name          string                 `json:"name"`
	ContainerPort int32                  `json:"containerPort"`
	Metadata      cueServiceSpecMetadata `json:"metadata"`
	Internal      bool                   `json:"internal"`
}

type cueServerBinary struct {
	Image string `json:"image"`
}

type cueServiceSpecMetadata struct {
	Kind     string `json:"kind"`
	Protocol string `json:"protocol"`
}

func parseCueServer(ctx context.Context, pl workspace.EarlyPackageLoader, loc workspace.Location, parent, v *fncue.CueV, pp *workspace.Package, opts workspace.LoadPackageOpts) (*schema.Server, error) {
	// Ensure all fields are bound.
	if err := v.Val.Validate(cue.Concrete(true)); err != nil {
		return nil, err
	}

	var bits cueServer
	if err := v.Val.Decode(&bits); err != nil {
		return nil, err
	}

	out := &schema.Server{}
	out.Id = bits.ID
	out.Name = bits.Name

	if v, ok := schema.Framework_value[bits.Framework]; ok {
		out.Framework = schema.Framework(v)
	} else {
		return nil, fnerrors.UserError(loc, "unrecognized framework: %s", bits.Framework)
	}

	if bits.Binary.Image != "" {
		if out.Framework == schema.Framework_OPAQUE {
			out.Binary = &schema.Server_Binary{
				Image: bits.Binary.Image,
			}
		} else {
			return nil, fnerrors.UserError(loc, "can't specify binary.image on non-opaque servers")
		}
	}

	out.IsStateful = bits.IsStateful
	out.Import = bits.Import
	out.StaticEnv = bits.StaticEnv

	var webServices schema.PackageList
	for _, entry := range bits.URLMap {
		if entry.Import != "" {
			webServices.Add(entry.Import)
		}
		out.UrlMap = append(out.UrlMap, &schema.Server_URLMapEntry{
			PathPrefix:  entry.Path,
			PackageName: string(entry.Import),
		})
	}

	out.Import = append(out.Import, webServices.PackageNamesAsString()...)

	for name, svc := range bits.Services {
		if svc.Metadata.Protocol == "" {
			return nil, fnerrors.UserError(loc, "service[%s]: a protocol is required", name)
		}

		out.Service = append(out.Service, &schema.Server_ServiceSpec{
			Name: name,
			Port: &schema.Endpoint_Port{Name: svc.Name, ContainerPort: svc.ContainerPort},
			Metadata: &schema.ServiceMetadata{
				Kind:     svc.Metadata.Kind,
				Protocol: svc.Metadata.Protocol,
			},
			Internal: svc.Internal,
		})
	}

	// Make services and endpoints stable.
	sort.Slice(out.Service, func(i, j int) bool {
		if out.Service[i].GetPort().GetContainerPort() == out.Service[j].GetPort().GetContainerPort() {
			return strings.Compare(out.Service[i].Name, out.Service[j].Name) < 0
		}
		return out.Service[i].GetPort().GetContainerPort() < out.Service[j].GetPort().GetContainerPort()
	})

	sort.Slice(out.Ingress, func(i, j int) bool {
		if out.Ingress[i].GetPort().GetContainerPort() == out.Ingress[j].GetPort().GetContainerPort() {
			return strings.Compare(out.Ingress[i].Name, out.Ingress[j].Name) < 0
		}
		return out.Ingress[i].GetPort().GetContainerPort() < out.Ingress[j].GetPort().GetContainerPort()
	})

	if err := fncue.WalkAttrs(parent.Val, func(v cue.Value, key, value string) error {
		switch key {
		case fncue.InputKeyword:
			if err := handleRef(loc, v, value, &out.Reference); err != nil {
				return err
			}

		case fncue.AllocKeyword:
			return fnerrors.UserError(loc, "servers don't support allocations, saw %q", value)
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return workspace.TransformServer(ctx, pl, loc, out, pp, opts)
}
