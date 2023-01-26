// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cuefrontend

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"cuelang.org/go/cue"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/frontend/cuefrontend/args"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

type cueServer struct {
	ID          string                     `json:"id"`
	Name        string                     `json:"name"`
	Description *schema.Server_Description `json:"description"`
	Framework   string                     `json:"framework"`
	IsStateful  bool                       `json:"isStateful"`
	PerNode     bool                       `json:"perNode"`
	TestOnly    bool                       `json:"testonly"`
	Import      []string                   `json:"import"`
	Services    map[string]cueServiceSpec  `json:"service"`
	Ingress     map[string]cueServiceSpec  `json:"ingress"`
	Env         *args.EnvMap               `json:"env"`
	Binary      interface{}                `json:"binary"` // Polymorphic: either package name, or cueServerBinary.

	// XXX this should be somewhere else.
	URLMap []cueURLMapEntry `json:"urlmap"`
}

type cueURLMapEntry struct {
	Path   string             `json:"path"`
	Import schema.PackageName `json:"import"`
}

type cueServiceSpec struct {
	Name                           string                   `json:"name"`
	Label                          string                   `json:"label"`
	ContainerPort                  int32                    `json:"containerPort"`
	Metadata                       cueServiceSpecMetadata   `json:"metadata"`
	Internal                       bool                     `json:"internal"`
	ExperimentalAdditionalMetadata []cueServiceSpecMetadata `json:"experimentalAdditionalMetadata"` // To consolidate with Metadata.
}

type cueServiceSpecMetadata struct {
	Kind                string        `json:"kind"`
	Protocol            string        `json:"protocol"`
	ExperimentalDetails inlineAnyJson `json:"experimentalDetails"`
}

type inlineAnyJson struct {
	TypeUrl string `json:"typeUrl"`
	Body    string `json:"body"`
}

// Returns generated binaries that need to be added to the package.
func parseCueServer(ctx context.Context, pl parsing.EarlyPackageLoader, loc pkggraph.Location, parent, v *fncue.CueV) (*schema.Server, []*schema.Binary, error) {
	// Ensure all fields are bound.
	if err := v.Val.Validate(cue.Concrete(true)); err != nil {
		return nil, nil, err
	}

	var bits cueServer
	if err := v.Val.Decode(&bits); err != nil {
		return nil, nil, err
	}

	var outBinaries []*schema.Binary

	out := &schema.Server{
		MainContainer: &schema.Container{},
	}
	out.Id = bits.ID
	out.Name = bits.Name
	out.Description = bits.Description

	if fmwk, err := parseFramework(loc, bits.Framework); err == nil {
		out.Framework = schema.Framework(fmwk)
	} else {
		return nil, nil, fnerrors.NewWithLocation(loc, "unrecognized framework: %s", bits.Framework)
	}

	if bits.Binary != nil {
		if out.Framework != schema.Framework_OPAQUE {
			return nil, nil, fnerrors.NewWithLocation(loc, "can't specify binary on non-opaque servers")
		}

		switch x := bits.Binary.(type) {
		case string:
			pkgRef, err := schema.ParsePackageRef(loc.PackageName, x)
			if err != nil {
				return nil, nil, fnerrors.NewWithLocation(loc, "invalid package reference: %s", x)
			}

			out.MainContainer.BinaryRef = pkgRef
		case map[string]interface{}:
			if image, ok := x["image"]; ok {
				// For prebuilt images, generating a binary in the server package and referring to it from the "Server.MainContainer".
				outBinaries = append(outBinaries, &schema.Binary{
					Name: bits.Name,
					BuildPlan: &schema.LayeredImageBuildPlan{
						LayerBuildPlan: []*schema.ImageBuildPlan{{ImageId: fmt.Sprintf("%s", image)}},
					},
				})
				out.MainContainer.BinaryRef = schema.MakePackageRef(loc.PackageName, bits.Name)
			} else {
				return nil, nil, fnerrors.NewWithLocation(loc, "binary: must either specify an image, or be a pointer to a binary package")
			}
		default:
			return nil, nil, fnerrors.InternalError("binary: unexpected type: %v", reflect.TypeOf(bits.Binary))
		}
	}

	if bits.IsStateful && bits.PerNode {
		return nil, nil, fnerrors.NewWithLocation(loc, "can't be both stateful and perNode")
	}

	if bits.PerNode {
		out.DeployableClass = string(schema.DeployableClass_DAEMONSET)
	} else if bits.IsStateful {
		out.DeployableClass = string(schema.DeployableClass_STATEFUL)
	} else {
		out.DeployableClass = string(schema.DeployableClass_STATELESS)
	}

	out.Testonly = bits.TestOnly
	out.Import = bits.Import
	out.RunByDefault = runByDefault(bits)

	env, err := bits.Env.Parsed(ctx, pl, loc.PackageName)
	if err != nil {
		return nil, nil, err
	}

	sort.Slice(env, func(i, j int) bool {
		x, y := env[i].Name, env[j].Name
		if x == y {
			return strings.Compare(env[i].Value, env[j].Value) < 0
		}
		return strings.Compare(x, y) < 0
	})

	out.MainContainer.Env = env

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
		parsed, err := parseService(loc, "service", name, svc)
		if err != nil {
			return nil, nil, err
		}

		out.Service = append(out.Service, parsed)
	}

	for name, svc := range bits.Ingress {
		if svc.Internal {
			return nil, nil, fnerrors.NewWithLocation(loc, "ingress[%s]: can't be internal", name)
		}

		parsed, err := parseService(loc, "ingress", name, svc)
		if err != nil {
			return nil, nil, err
		}

		out.Ingress = append(out.Ingress, parsed)
	}

	if err := fncue.WalkAttrs(parent.Val, func(v cue.Value, key, value string) error {
		switch key {
		case fncue.InputKeyword:
			if err := handleRef(loc, v, value, &out.Reference); err != nil {
				return err
			}

		case fncue.AllocKeyword:
			return fnerrors.NewWithLocation(loc, "servers don't support allocations, saw %q", value)
		}

		return nil
	}); err != nil {
		return nil, nil, err
	}

	return out, outBinaries, nil
}

func parseDetails(detail inlineAnyJson) (*anypb.Any, error) {
	if detail.TypeUrl == "" {
		return nil, nil
	}

	any := &anypb.Any{TypeUrl: detail.TypeUrl}
	msg, err := any.UnmarshalNew()
	if err != nil {
		return nil, err
	}
	if err := protojson.Unmarshal([]byte(detail.Body), msg); err != nil {
		return nil, err
	}
	return anypb.New(msg)
}

func parseService(loc pkggraph.Location, kind, name string, svc cueServiceSpec) (*schema.Server_ServiceSpec, error) {
	if svc.Metadata.ExperimentalDetails.TypeUrl != "" {
		return nil, fnerrors.NewWithLocation(loc, "%s[%s]: only additional metadata support details", kind, name)
	}

	if svc.Metadata.Protocol == "" {
		return nil, fnerrors.NewWithLocation(loc, "%s[%s]: a protocol is required", kind, name)
	}

	parsed := &schema.Server_ServiceSpec{
		Name:  name,
		Label: svc.Label,
		Port:  &schema.Endpoint_Port{Name: svc.Name, ContainerPort: svc.ContainerPort},
		Metadata: []*schema.ServiceMetadata{{
			Kind:     svc.Metadata.Kind,
			Protocol: svc.Metadata.Protocol,
		}},
		Internal: svc.Internal,
	}

	for _, add := range svc.ExperimentalAdditionalMetadata {
		details, err := parseDetails(add.ExperimentalDetails)
		if err != nil {
			return nil, fnerrors.NewWithLocation(loc, "%s[%s]: failed to parse: %w", kind, name, err)
		}
		parsed.Metadata = append(parsed.Metadata, &schema.ServiceMetadata{Kind: add.Kind, Protocol: add.Protocol, Details: details})
	}

	return parsed, nil
}

func runByDefault(bits cueServer) bool {
	if bits.TestOnly {
		return false
	}

	// Skip the orchestration server by default.
	// TODO scale this if we see a need.
	if bits.ID == "0fomj22adbua2u0ug3og" {
		return false
	}

	return true
}
