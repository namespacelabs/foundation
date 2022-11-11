// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cuefrontendopaque

import (
	"golang.org/x/exp/maps"
	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

type cueService struct {
	Kind    string     `json:"kind"`
	Port    int        `json:"port"`
	Ingress cueIngress `json:"ingress"`

	ReadinessProbe map[string]string            `json:"probe"`  // `probe: http: "/"`
	Probes         map[string]map[string]string `json:"probes"` // `probes: readiness: http: "/"`
}

type cueIngress struct {
	InternetFacing bool                `json:"internetFacing"`
	HttpRoutes     map[string][]string `json:"httpRoutes"`
}

func parseService(loc pkggraph.Location, name string, svc cueService) (*schema.Server_ServiceSpec, schema.Endpoint_Type, error) {
	if svc.Kind != "http" && svc.Kind != "tcp" {
		return nil, schema.Endpoint_INGRESS_UNSPECIFIED, fnerrors.NewWithLocation(loc, "service kind is not supported: %s", svc.Kind)
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

	if svc.Probes != nil && svc.ReadinessProbe != nil {
		return nil, schema.Endpoint_INGRESS_UNSPECIFIED, fnerrors.BadInputError("probes and probe are exclusive")
	}

	if svc.ReadinessProbe != nil {
		md, err := parseProbe(runtime.FnServiceReadyz, svc.ReadinessProbe)
		if err != nil {
			return nil, schema.Endpoint_INGRESS_UNSPECIFIED, err
		}
		parsed.Metadata = append(parsed.Metadata, md)
	}

	for name, data := range svc.Probes {
		var kind string
		switch name {
		case "readiness":
			kind = runtime.FnServiceReadyz
		case "liveness":
			kind = runtime.FnServiceLivez
		default:
			return nil, schema.Endpoint_INGRESS_UNSPECIFIED, fnerrors.BadInputError("%s: unsupported probe kind", name)
		}

		md, err := parseProbe(kind, data)
		if err != nil {
			return nil, schema.Endpoint_INGRESS_UNSPECIFIED, err
		}
		parsed.Metadata = append(parsed.Metadata, md)
	}

	return parsed, endpointType, nil
}

func parseProbe(kind string, data map[string]string) (*schema.ServiceMetadata, error) {
	for _, key := range maps.Keys(data) {
		if key == "http" {
			httpmd := &schema.HttpExportedService{Path: data[key]}
			serializedHttpmd, err := anypb.New(httpmd)
			if err != nil {
				return nil, fnerrors.InternalError("failed to serialize ServiceMetadata")
			}
			return &schema.ServiceMetadata{Kind: kind, Details: serializedHttpmd}, nil
		} else {
			return nil, fnerrors.BadInputError("%s: unsupported probe type", key)
		}
	}

	return nil, nil
}
