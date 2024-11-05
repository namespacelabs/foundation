// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package planning

import (
	"fmt"

	"golang.org/x/exp/slices"
	anypb "google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/planning/constants"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

type EndpointProvider interface {
	InternalEndpoints(*schema.Environment, *schema.Server, []*schema.Endpoint_Port) ([]*schema.InternalEndpoint, error)
}

var endpointProviderByFramework = map[string]EndpointProvider{}

// XXX this is not the right place for protocol handling.
func RegisterEndpointProvider(fmwk schema.Framework, f EndpointProvider) {
	endpointProviderByFramework[fmwk.String()] = f
}

func ComputeEndpoints(planner runtime.Planner, srv Server, merged *schema.ServerFragment, serverPorts []*schema.Endpoint_Port) ([]*schema.Endpoint, []*schema.InternalEndpoint, error) {
	sch := srv.StackEntry()

	// XXX figure out a story to handle collisions within a server!
	// XXX should this be by exported RPC service instead?

	var endpoints []*schema.Endpoint

	for _, service := range sch.Services() {
		var pkg *pkggraph.Package
		for _, p := range srv.Deps() {
			if p.PackageName().Equals(service.PackageName) {
				pkg = p
				break
			}
		}

		var lst *schema.Listener
		if service.ListenerName != "" {
			for _, l := range merged.Listener {
				if l.Name == service.ListenerName {
					lst = l
				}
			}

			if lst == nil {
				return nil, nil, fnerrors.New("service %q refers to non-existing listener %q", pkg.PackageName(), service.ListenerName)
			}
		} else {
			serverPort := findPort(serverPorts, "server-port")
			if serverPort == nil {
				return nil, nil, fnerrors.New("listener %q is missing a corresponding port", service.ListenerName)
			}

			lst = &schema.Listener{
				Port: serverPort,
			}
		}

		nd, err := computeServiceEndpoint(planner, sch.Server, lst, pkg, service, service.GetIngress())
		if err != nil {
			return nil, nil, err
		}

		endpoints = append(endpoints, nd...)
	}

	// Handle statically defined services.
	server := sch.Server
	for _, s := range merged.GetService() {
		t := schema.Endpoint_PRIVATE
		if s.EndpointType != schema.Endpoint_INGRESS_UNSPECIFIED {
			t = s.EndpointType
		}

		spec, err := ServiceSpecToEndpoint(planner, server, s, t)
		if err != nil {
			return nil, nil, err
		}
		endpoints = append(endpoints, spec)
	}

	for _, s := range merged.GetIngress() {
		if s.EndpointType != schema.Endpoint_INGRESS_UNSPECIFIED && s.EndpointType != schema.Endpoint_INTERNET_FACING {
			return nil, nil, fnerrors.InternalError("ingress endpoint type is incompatible, saw %v", s.EndpointType)
		}

		spec, err := ServiceSpecToEndpoint(planner, server, s, schema.Endpoint_INTERNET_FACING)
		if err != nil {
			return nil, nil, err
		}
		endpoints = append(endpoints, spec)
	}

	// Handle HTTP.
	if needsHTTP := len(server.UrlMap) > 0; needsHTTP {
		httpPort := findPort(serverPorts, "http-port")

		short, fqdn := planner.MakeServiceName(server.Name)

		// We need a http service to hit.
		endpoints = append(endpoints, &schema.Endpoint{
			Type:        schema.Endpoint_PRIVATE,
			ServiceName: constants.HttpServiceName,
			Ports: []*schema.Endpoint_PortMap{
				{ExportedPort: httpPort.GetContainerPort(), Port: httpPort},
			},
			AllocatedName:      short,
			FullyQualifiedName: fqdn,
			EndpointOwner:      server.GetPackageName(),
			ServerOwner:        server.GetPackageName(),
			ServiceMetadata: []*schema.ServiceMetadata{
				{Protocol: "http"},
			},
		})
	}

	var internal []*schema.InternalEndpoint

	if f, ok := endpointProviderByFramework[server.Framework.String()]; ok {
		var err error
		internal, err = f.InternalEndpoints(srv.SealedContext().Environment(), server, serverPorts)
		if err != nil {
			return nil, nil, err
		}
	}

	return endpoints, internal, nil
}

func findPort(serverPorts []*schema.Endpoint_Port, name string) *schema.Endpoint_Port {
	for _, port := range serverPorts {
		if port.Name == name {
			return port
		}
	}

	return nil
}

// XXX this should be somewhere else.
func computeServiceEndpoint(planner runtime.Planner, server *schema.Server, listener *schema.Listener, pkg *pkggraph.Package, n *schema.Node, t schema.Endpoint_Type) ([]*schema.Endpoint, error) {
	var endpoints []*schema.Endpoint

	exportedPort := n.ExportedPort
	if exportedPort == 0 {
		exportedPort = listener.Port.ContainerPort
	}

	if len(n.ExportService) == 0 {
		if n.ListenerName != "" {
			// XXX should we perhaps export an endpoint per service.
			name := n.GetIngressServiceName()
			short, fqdn := planner.MakeServiceName(name)

			endpoints = append(endpoints, &schema.Endpoint{
				ServiceName:        name,
				AllocatedName:      short,
				FullyQualifiedName: fqdn,
				EndpointOwner:      n.GetPackageName(),
				ServerOwner:        server.GetPackageName(),
				Type:               t,
				Ports:              []*schema.Endpoint_PortMap{{ExportedPort: exportedPort, Port: listener.Port}},
			})
		}
	} else {
		for k, exported := range n.ExportService {
			name := n.GetIngressServiceName()
			if k > 0 {
				name += fmt.Sprintf("-%d", k)
			}

			// XXX should we perhaps export an endpoint per service.
			short, fqdn := planner.MakeServiceName(name + "-grpc")

			endpoint := &schema.Endpoint{
				ServiceName:        name,
				AllocatedName:      short,
				FullyQualifiedName: fqdn,
				EndpointOwner:      n.GetPackageName(),
				ServerOwner:        server.GetPackageName(),
				Type:               t,
				Ports:              []*schema.Endpoint_PortMap{{ExportedPort: exportedPort, Port: listener.Port}},
			}

			if slices.Contains(constants.ReservedServiceNames, endpoint.ServiceName) {
				return nil, fnerrors.InternalError("%s: %q is a reserved service name", n.PackageName, endpoint.ServiceName)
			}

			details, err := anypb.New(exported)
			if err != nil {
				return nil, err
			}

			endpoint.ServiceMetadata = append(endpoint.ServiceMetadata, &schema.ServiceMetadata{
				Kind:     exported.ProtoTypename,
				Protocol: schema.ClearTextGrpcProtocol,
				Details:  details,
			})

			endpoints = append(endpoints, endpoint)
		}
	}

	return endpoints, nil
}

func ServiceSpecToEndpoint(planner runtime.Planner, srv *schema.Server, spec *schema.Server_ServiceSpec, t schema.Endpoint_Type) (*schema.Endpoint, error) {
	short, fqdn := planner.MakeServiceName(fmt.Sprintf("%s-%s", spec.GetName(), srv.Id))

	endpoint := &schema.Endpoint{
		ServiceName:        spec.GetName(),
		ServerOwner:        srv.GetPackageName(),
		EndpointOwner:      srv.GetPackageName(),
		Type:               t,
		AllocatedName:      short,
		FullyQualifiedName: fqdn,
		ServiceLabel:       spec.GetLabel(),
		ServiceMetadata:    spec.Metadata,
		IngressProvider:    spec.IngressProvider,
		Headless:           spec.Headless,
	}

	ingressSpec := &schema.Endpoint_IngressSpec{}

	if len(spec.IngressDomain) > 0 {
		if t != schema.Endpoint_INTERNET_FACING {
			return nil, fnerrors.InternalError("ingress domain is specified in non-ingress endpoint")
		}

		ingressSpec.Domain = spec.IngressDomain
	}

	ingressSpec.Annotations = spec.IngressAnnotations
	endpoint.IngressSpec = ingressSpec

	for _, port := range spec.GetPorts() {
		if port.Port == nil {
			continue
		}

		exportedPort := port.ExportedPort
		if exportedPort == 0 {
			exportedPort = port.Port.GetContainerPort()
		}

		endpoint.Ports = append(endpoint.Ports, &schema.Endpoint_PortMap{
			ExportedPort: exportedPort,
			Port:         port.Port,
		})
	}

	// XXX Rethink this -- i.e. consolidate with InternalEndpoint.
	if spec.Internal {
		endpoint.ServiceMetadata = append(endpoint.ServiceMetadata, &schema.ServiceMetadata{
			Kind: constants.ManualInternalService,
		})
	}

	return endpoint, nil
}
