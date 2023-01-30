// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cuefrontendopaque

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"

	"golang.org/x/exp/slices"
	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

// Needs to be consistent with JSON names of cueResourceClass fields.
var serviceFields = []string{"kind", "port", "exportedPort", "ingress", "annotations", "probe", "probes"}

type cueService struct {
	Kind         string     `json:"kind"`
	Port         int32      `json:"port"`
	ExportedPort int32      `json:"exportedPort"`
	Ingress      cueIngress `json:"ingress"`

	Annotations map[string]string `json:"annotations,omitempty"`

	ReadinessProbe *cueServiceProbe            `json:"probe"`  // `probe: http: "/"`
	Probes         map[string]*cueServiceProbe `json:"probes"` // `probes: readiness: http: "/"`
}

type cueServiceProbe struct {
	Path string `json:"http"`
}

type cueIngress struct {
	Enabled      bool
	LoadBalancer bool
	Details      CueIngressDetails
}

type CueIngressDetails struct {
	// Key is domain.
	HttpRoutes     map[string][]string `json:"httpRoutes"`
	ProviderClass  string              `json:"provider"`
	Domains        []string            `json:"domains"`
	AllowedOrigins []string            `json:"allowed_origins"`
}

var _ json.Unmarshaler = &cueIngress{}

func (i *cueIngress) UnmarshalJSON(contents []byte) error {
	if contents == nil {
		return nil
	}

	dec := json.NewDecoder(bytes.NewReader(contents))

	tok, err := dec.Token()
	if err != nil {
		return err
	}

	if tok == json.Delim('{') {
		if err := json.Unmarshal(contents, &i.Details); err != nil {
			return err
		}

		i.Enabled = true
		return nil
	} else if str, ok := tok.(string); ok {
		switch strings.ToLower(str) {
		case "true":
			i.Enabled = true
			return nil

		case "loadbalancer":
			i.LoadBalancer = true
			return nil

		default:
			return fnerrors.New("ingress: expected 'true', 'LoadBalancer', or a full ingress definition got %q", str)
		}
	} else if b, ok := tok.(bool); ok {
		if b {
			i.Enabled = true
			return nil
		}
		return fnerrors.New("ingress: expected 'true', 'LoadBalancer', or a full ingress definition got \"%v\"", b)
	} else {
		return fnerrors.New("ingress: bad value %v, expected 'true', 'LoadBalancer', or a full ingress definition", tok)
	}
}

var knownKinds = []string{"tcp", schema.ClearTextGrpcProtocol, schema.GrpcProtocol, schema.HttpProtocol, schema.HttpsProtocol}

func parseService(ctx context.Context, pl pkggraph.PackageLoader, loc pkggraph.Location, name string, svc cueService) (*schema.Server_ServiceSpec, []*schema.Probe, error) {
	if svc.Kind != "" && !slices.Contains(knownKinds, svc.Kind) {
		return nil, nil, fnerrors.NewWithLocation(loc, "service kind is not supported: %s (support %v)", svc.Kind, strings.Join(knownKinds, ", "))
	}

	var endpointType schema.Endpoint_Type
	if svc.Ingress.Enabled {
		endpointType = schema.Endpoint_INTERNET_FACING
	} else if svc.Ingress.LoadBalancer {
		if err := parsing.RequireFeature(loc.Module, "experimental/service/loadbalancer"); err != nil {
			return nil, nil, err
		}
		endpointType = schema.Endpoint_LOAD_BALANCER
	} else {
		endpointType = schema.Endpoint_PRIVATE
	}

	urlMap := &schema.HttpUrlMap{}
	for domain, routes := range svc.Ingress.Details.HttpRoutes {
		if domain != "*" {
			return nil, nil, fnerrors.New("unsupported domain, only support * for now")
		}

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
			return nil, nil, err
		}
	}

	// For the time being, having a grpc service implies exporting all GRPC services.
	if svc.Kind == schema.GrpcProtocol || svc.Kind == schema.ClearTextGrpcProtocol {
		if details != nil {
			return nil, nil, fnerrors.New("service metadata was already set")
		}

		details = &anypb.Any{}
		if err := details.MarshalFrom(&schema.GrpcExportAllServices{}); err != nil {
			return nil, nil, fnerrors.New("failed to serialize grpc configuration: %w", err)
		}
	}

	parsed := &schema.Server_ServiceSpec{
		Name:         name,
		Port:         &schema.Endpoint_Port{Name: name, ContainerPort: svc.Port},
		ExportedPort: svc.ExportedPort,
		EndpointType: endpointType,
	}

	if svc.Kind != "" {
		parsed.Metadata = append(parsed.Metadata, &schema.ServiceMetadata{Protocol: svc.Kind, Details: details})
	} else if details != nil {
		return nil, nil, fnerrors.New("service metadata is specified without kind")
	}

	if len(svc.Ingress.Details.AllowedOrigins) > 0 {
		if svc.Kind != schema.HttpProtocol {
			return nil, nil, fnerrors.New("can only specify CORS when protocol is http")
		}

		cors := &schema.HttpCors{Enabled: true, AllowedOrigin: svc.Ingress.Details.AllowedOrigins}
		packedCors, err := anypb.New(cors)
		if err != nil {
			return nil, nil, fnerrors.New("failed to pack CORS' configuration: %v", err)
		}

		parsed.Metadata = append(parsed.Metadata, &schema.ServiceMetadata{
			Kind:    "http-extension",
			Details: packedCors,
		})
	}

	if svc.Ingress.Details.ProviderClass != "" {
		ref, err := pkggraph.ParseAndLoadRef(ctx, pl, loc, svc.Ingress.Details.ProviderClass)
		if err != nil {
			return nil, nil, err
		}

		parsed.IngressProvider = ref
	}

	for _, domain := range svc.Ingress.Details.Domains {
		parsed.IngressDomain = append(parsed.IngressDomain, &schema.DomainSpec{
			Fqdn:    domain,
			Managed: schema.Domain_USER_SPECIFIED_TLS_MANAGED,
		})
	}

	if len(svc.Annotations) > 0 {
		if err := parsing.RequireFeature(loc.Module, "experimental/service/annotations"); err != nil {
			return nil, nil, err
		}

		x := &schema.ServiceAnnotations{}
		for key, value := range svc.Annotations {
			x.KeyValue = append(x.KeyValue, &schema.ServiceAnnotations_KeyValue{Key: key, Value: value})
		}
		slices.SortFunc(x.KeyValue, func(a, b *schema.ServiceAnnotations_KeyValue) bool {
			if a.Key == b.Key {
				return strings.Compare(a.Value, b.Value) < 0
			}
			return strings.Compare(a.Key, b.Key) < 0
		})

		serialized, err := anypb.New(x)
		if err != nil {
			return nil, nil, fnerrors.InternalError("failed to serialize annotations: %w", err)
		}

		parsed.Metadata = append(parsed.Metadata, &schema.ServiceMetadata{
			Details: serialized,
		})
	}

	if svc.Probes != nil && svc.ReadinessProbe != nil {
		return nil, nil, fnerrors.AttachLocation(loc, fnerrors.BadInputError("probes and probe are exclusive"))
	}

	if svc.ReadinessProbe != nil {
		probe := &schema.Probe{
			Kind: runtime.FnServiceReadyz,
			Http: &schema.Probe_Http{
				ContainerPort: svc.Port,
				Path:          svc.ReadinessProbe.Path,
			},
		}

		return parsed, []*schema.Probe{probe}, nil
	}

	var probes []*schema.Probe
	for name, data := range svc.Probes {
		kind, err := parseProbeKind(name)
		if err != nil {
			return nil, nil, fnerrors.AttachLocation(loc, err)
		}

		probes = append(probes, &schema.Probe{
			Kind: kind,
			Http: &schema.Probe_Http{
				ContainerPort: svc.Port,
				Path:          data.Path,
			},
		})
	}

	return parsed, probes, nil
}
