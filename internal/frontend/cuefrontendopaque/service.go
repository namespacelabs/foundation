// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cuefrontendopaque

import (
	"bytes"
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

	ReadinessProbe *cueProbe            `json:"probe"`  // `probe: http: "/"`
	Probes         map[string]*cueProbe `json:"probes"` // `probes: readiness: http: "/"`
}

type cueProbe struct {
	Http *cueHttpProbe `json:"http"`
	Exec *cueExecProbe `json:"exec"`
}

type cueHttpProbe struct {
	Path string
}

var _ json.Unmarshaler = &cueHttpProbe{}

func (h *cueHttpProbe) UnmarshalJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	tok, err := d.Token()
	if err != nil {
		return err
	}

	if str, ok := tok.(string); ok {
		h.Path = str
		return nil
	}

	return fnerrors.BadInputError("failed to parse http probe, unexpected token %v", tok)
}

type cueExecProbe struct {
	Command []string
}

var _ json.Unmarshaler = &cueExecProbe{}

func (e *cueExecProbe) UnmarshalJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	tok, err := d.Token()
	if err != nil {
		return err
	}

	if str, ok := tok.(string); ok {
		e.Command = []string{str}
		return nil
	}

	if tok == json.Delim('[') {
		return json.Unmarshal(data, &e.Command)
	}

	return fnerrors.BadInputError("failed to parse exec probe, unexpected token %v", tok)
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

func parseService(loc pkggraph.Location, name string, svc cueService) (*schema.Server_ServiceSpec, schema.Endpoint_Type, error) {
	if !slices.Contains(knownKinds, svc.Kind) {
		return nil, schema.Endpoint_INGRESS_UNSPECIFIED, fnerrors.NewWithLocation(loc, "service kind is not supported: %s (support %v)", svc.Kind, strings.Join(knownKinds, ", "))
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
			return nil, schema.Endpoint_INGRESS_UNSPECIFIED, err
		}
	}

	// For the time being, having a grpc service implies exporting all GRPC services.
	if svc.Kind == schema.GrpcProtocol || svc.Kind == schema.ClearTextGrpcProtocol {
		details = &anypb.Any{}
		if err := details.MarshalFrom(&schema.GrpcExportAllServices{}); err != nil {
			return nil, schema.Endpoint_INGRESS_UNSPECIFIED, fnerrors.New("failed to serialize grpc configuration: %w", err)
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

func parseProbe(kind string, probe *cueProbe) (*schema.ServiceMetadata, error) {
	if probe.Http != nil && probe.Exec != nil {
		return nil, fnerrors.BadInputError("probe: `http` and `exec` may not be both set")
	}

	switch {
	case probe.Http != nil:
		md, err := anypb.New(&schema.HttpExportedService{Path: probe.Http.Path})
		if err != nil {
			return nil, fnerrors.InternalError("failed to serialize HttpExportedService")
		}

		return &schema.ServiceMetadata{Kind: kind, Details: md}, nil

	case probe.Exec != nil:
		md, err := anypb.New(&schema.ExecProbe{Command: probe.Exec.Command})
		if err != nil {
			return nil, fnerrors.InternalError("failed to serialize ExecProbe")
		}

		return &schema.ServiceMetadata{Kind: kind, Details: md}, nil
	}

	return nil, nil
}
