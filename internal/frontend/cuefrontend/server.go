// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cuefrontend

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"cuelang.org/go/cue"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
)

type cueServer struct {
	ID           string                     `json:"id"`
	Name         string                     `json:"name"`
	Description  *schema.Server_Description `json:"description"`
	Framework    string                     `json:"framework"`
	IsStateful   bool                       `json:"isStateful"`
	TestOnly     bool                       `json:"testonly"`
	ClusterAdmin bool                       `json:"clusterAdmin"`
	Import       []string                   `json:"import"`
	Services     map[string]cueServiceSpec  `json:"service"`
	StaticEnv    map[string]string          `json:"env"`
	Binary       interface{}                `json:"binary"` // Polymorphic: either package name, or cueServerBinary.

	// XXX this should be somewhere else.
	URLMap []cueURLMapEntry `json:"urlmap"`
}

type cueURLMapEntry struct {
	Path   string             `json:"path"`
	Import schema.PackageName `json:"import"`
}

type cueServiceSpec struct {
	Name          string                 `json:"name"`
	Label         string                 `json:"label"`
	ContainerPort int32                  `json:"containerPort"`
	Metadata      cueServiceSpecMetadata `json:"metadata"`
	Internal      bool                   `json:"internal"`
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
	out.Description = bits.Description

	if fmwk, err := parseFramework(loc, bits.Framework); err == nil {
		out.Framework = schema.Framework(fmwk)
	} else {
		return nil, fnerrors.UserError(loc, "unrecognized framework: %s", bits.Framework)
	}

	if bits.Binary != nil {
		if out.Framework != schema.Framework_OPAQUE {
			return nil, fnerrors.UserError(loc, "can't specify binary on non-opaque servers")
		}

		switch x := bits.Binary.(type) {
		case string:
			out.Binary = &schema.Server_Binary{
				PackageName: x,
			}
		case map[string]interface{}:
			if image, ok := x["image"]; ok {
				out.Binary = &schema.Server_Binary{
					Prebuilt: fmt.Sprintf("%s", image),
				}
			} else {
				return nil, fnerrors.UserError(loc, "binary: must either specify an image, or be a pointer to a binary package")
			}
		default:
			return nil, fnerrors.InternalError("binary: unexpected type: %v", reflect.TypeOf(bits.Binary))
		}
	}

	out.IsStateful = bits.IsStateful
	out.Testonly = bits.TestOnly
	out.ClusterAdmin = bits.ClusterAdmin
	out.Import = bits.Import

	for k, v := range bits.StaticEnv {
		out.StaticEnv = append(out.StaticEnv, &schema.BinaryConfig_EnvEntry{
			Name: k, Value: v,
		})
	}
	sort.Slice(out.StaticEnv, func(i, j int) bool {
		x, y := out.StaticEnv[i].Name, out.StaticEnv[j].Name
		if x == y {
			return strings.Compare(out.StaticEnv[i].Value, out.StaticEnv[j].Value) < 0
		}
		return strings.Compare(x, y) < 0
	})

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
			Name:  name,
			Label: svc.Label,
			Port:  &schema.Endpoint_Port{Name: svc.Name, ContainerPort: svc.ContainerPort},
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
