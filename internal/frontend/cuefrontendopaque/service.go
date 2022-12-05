// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cuefrontendopaque

import (
	"encoding/json"
	"strings"

	"google.golang.org/protobuf/types/known/anypb"
	"k8s.io/utils/strings/slices"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

type cueService struct {
	Kind    string     `json:"kind"`
	Port    int        `json:"port"`
	Ingress cueIngress `json:"ingress"`

	ReadinessProbe *cueServiceProbe            `json:"probe"`  // `probe: http: "/"`
	Probes         map[string]*cueServiceProbe `json:"probes"` // `probes: readiness: http: "/"`
}

type cueServiceProbe struct {
	Path string `json:"http"`
}

type cueIngress struct {
	Enabled bool
	Details CueIngressDetails
}

type CueIngressDetails struct {
	HttpRoutes map[string][]string `json:"httpRoutes"`
}

var _ json.Unmarshaler = &cueIngress{}

func (i *cueIngress) UnmarshalJSON(contents []byte) error {
	if contents == nil {
		return nil
	}

	if string(contents) == "true" {
		i.Enabled = true
		return nil
	}

	if json.Unmarshal(contents, &i.Details) == nil {
		i.Enabled = true
		return nil
	}

	return fnerrors.InternalError("ingress: expected 'true', or a full ingress definition")
}

var knownKinds = []string{"tcp", schema.ClearTextGrpcProtocol, schema.GrpcProtocol, schema.HttpProtocol}

func parseService(loc pkggraph.Location, name string, svc cueService) (*schema.Server_ServiceSpec, schema.Endpoint_Type, []*schema.Probe, error) {
	if !slices.Contains(knownKinds, svc.Kind) {
		return nil, schema.Endpoint_INGRESS_UNSPECIFIED, nil, fnerrors.NewWithLocation(loc, "service kind is not supported: %s (support %v)", svc.Kind, strings.Join(knownKinds, ", "))
	}

	var endpointType schema.Endpoint_Type
	if svc.Ingress.Enabled {
		endpointType = schema.Endpoint_INTERNET_FACING
	} else {
		endpointType = schema.Endpoint_PRIVATE
	}

	urlMap := &schema.HttpUrlMap{}
	for _, routes := range svc.Ingress.Details.HttpRoutes {
		for _, route := range routes {
			urlMap.Entry = append(urlMap.Entry, &schema.HttpUrlMap_Entry{
				PathPrefix: route,
			})
		}
	}
	var details *anypb.Any
	if len(urlMap.Entry) > 0 {
		details = &anypb.Any{}
		if err := details.MarshalFrom(urlMap); err != nil {
			return nil, schema.Endpoint_INGRESS_UNSPECIFIED, nil, err
		}
	}

	// For the time being, having a grpc service implies exporting all GRPC services.
	if svc.Kind == schema.GrpcProtocol || svc.Kind == schema.ClearTextGrpcProtocol {
		details = &anypb.Any{}
		if err := details.MarshalFrom(&schema.GrpcExportAllServices{}); err != nil {
			return nil, schema.Endpoint_INGRESS_UNSPECIFIED, nil, fnerrors.New("failed to serialize grpc configuration: %w", err)
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
		return nil, schema.Endpoint_INGRESS_UNSPECIFIED, nil, fnerrors.AttachLocation(loc, fnerrors.BadInputError("probes and probe are exclusive"))
	}

	if svc.ReadinessProbe != nil {
		probe := &schema.Probe{
			Kind: runtime.FnServiceReadyz,
			Http: &schema.Probe_Http{
				ContainerPort: int32(svc.Port),
				Path:          svc.ReadinessProbe.Path,
			},
		}

		return parsed, endpointType, []*schema.Probe{probe}, nil
	}

	var probes []*schema.Probe
	for name, data := range svc.Probes {
		kind, err := parseProbeKind(name)
		if err != nil {
			return nil, schema.Endpoint_INGRESS_UNSPECIFIED, nil, fnerrors.AttachLocation(loc, err)
		}

		probes = append(probes, &schema.Probe{
			Kind: kind,
			Http: &schema.Probe_Http{
				ContainerPort: int32(svc.Port),
				Path:          data.Path,
			},
		})
	}

	return parsed, endpointType, probes, nil
}
