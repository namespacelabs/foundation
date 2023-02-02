// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package planning

import (
	"fmt"

	"golang.org/x/exp/slices"
	"google.golang.org/protobuf/types/descriptorpb"
	anypb "google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/planning/constants"
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

func ComputeEndpoints(srv Server, merged *schema.ServerFragment, allocatedPorts []*schema.Endpoint_Port) ([]*schema.Endpoint, []*schema.InternalEndpoint, error) {
	sch := srv.StackEntry()
	serverPorts := append([]*schema.Endpoint_Port{}, sch.Server.StaticPort...)
	serverPorts = append(serverPorts, allocatedPorts...)

	// XXX figure out a story to handle collisions within a server!
	// XXX should this be by exported RPC service instead?

	var endpoints []*schema.Endpoint

	var serverPort *schema.Endpoint_Port
	for _, port := range serverPorts {
		if port.Name == "server-port" { // XXX this needs to be thought through, it's convention by naming.
			serverPort = port
			break
		}
	}

	for _, service := range sch.Services() {
		var pkg *pkggraph.Package
		for _, p := range srv.Deps() {
			if p.PackageName().Equals(service.PackageName) {
				pkg = p
				break
			}
		}

		nd, err := computeServiceEndpoint(sch.Server, pkg, service, service.GetIngress(), serverPort)
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

		spec, err := ServiceSpecToEndpoint(server, s, t)
		if err != nil {
			return nil, nil, err
		}
		endpoints = append(endpoints, spec)
	}

	for _, s := range merged.GetIngress() {
		if s.EndpointType != schema.Endpoint_INGRESS_UNSPECIFIED && s.EndpointType != schema.Endpoint_INTERNET_FACING {
			return nil, nil, fnerrors.InternalError("ingress endpoint type is incompatible, saw %v", s.EndpointType)
		}

		spec, err := ServiceSpecToEndpoint(server, s, schema.Endpoint_INTERNET_FACING)
		if err != nil {
			return nil, nil, err
		}
		endpoints = append(endpoints, spec)
	}

	// Handle HTTP.
	if needsHTTP := len(server.UrlMap) > 0; needsHTTP {
		var httpPort *schema.Endpoint_Port
		for _, port := range serverPorts {
			if port.Name == "http-port" {
				httpPort = port
				break
			}
		}

		// We need a http service to hit.
		endpoints = append(endpoints, &schema.Endpoint{
			Type:          schema.Endpoint_PRIVATE,
			ServiceName:   constants.HttpServiceName,
			Port:          httpPort,
			ExportedPort:  httpPort.GetContainerPort(),
			AllocatedName: server.Name,
			EndpointOwner: server.GetPackageName(),
			ServerOwner:   server.GetPackageName(),
			ServiceMetadata: []*schema.ServiceMetadata{
				{Protocol: "http"},
			},
		})
	}

	var internal []*schema.InternalEndpoint

	if f, ok := endpointProviderByFramework[server.Framework.String()]; ok {
		var err error
		internal, err = f.InternalEndpoints(srv.SealedContext().Environment(), server, allocatedPorts)
		if err != nil {
			return nil, nil, err
		}
	}

	return endpoints, internal, nil
}

// XXX this should be somewhere else.
func computeServiceEndpoint(server *schema.Server, pkg *pkggraph.Package, n *schema.Node, t schema.Endpoint_Type, serverPort *schema.Endpoint_Port) ([]*schema.Endpoint, error) {
	if len(n.ExportService) == 0 {
		return nil, nil
	}

	// XXX should we perhaps export an endpoint per service.

	endpoint := &schema.Endpoint{
		ServiceName:   n.GetIngressServiceName(),
		AllocatedName: n.GetIngressServiceName() + "-grpc",
		EndpointOwner: n.GetPackageName(),
		ServerOwner:   server.GetPackageName(),
		Type:          t,
		Port:          serverPort,
		ExportedPort:  serverPort.GetContainerPort(),
	}

	if slices.Contains(constants.ReservedServiceNames, endpoint.ServiceName) {
		return nil, fnerrors.InternalError("%s: %q is a reserved service name", n.PackageName, endpoint.ServiceName)
	}

	for _, exported := range n.ExportService {
		details, err := anypb.New(exported)
		if err != nil {
			return nil, err
		}

		endpoint.ServiceMetadata = append(endpoint.ServiceMetadata, &schema.ServiceMetadata{
			Kind:     exported.ProtoTypename,
			Protocol: schema.ClearTextGrpcProtocol,
			Details:  details,
		})

		if n.ExportServicesAsHttp {
			details, err := anypb.New(&schema.GrpcExportService{ProtoTypename: exported.ProtoTypename})
			if err != nil {
				return nil, err
			}

			endpoint.ServiceMetadata = append(endpoint.ServiceMetadata, &schema.ServiceMetadata{
				Kind:    constants.KindNeedsGrpcGateway,
				Details: details,
			})

			if pkg != nil {
				if def := pkg.Services[exported.ProtoTypename]; def != nil {
					fds := &descriptorpb.FileDescriptorSet{}
					fds.File = append(fds.File, def.GetFile()...)
					fds.File = append(fds.File, def.GetDependency()...)

					details, err := anypb.New(&schema.GrpcHttpTranscoding{
						FileDescriptorSet: fds,
					})
					if err != nil {
						return nil, err
					}

					endpoint.ServiceMetadata = append(endpoint.ServiceMetadata, &schema.ServiceMetadata{Details: details})
				}
			}
		}
	}

	return []*schema.Endpoint{endpoint}, nil
}

func ServiceSpecToEndpoint(srv *schema.Server, spec *schema.Server_ServiceSpec, t schema.Endpoint_Type) (*schema.Endpoint, error) {
	endpoint := &schema.Endpoint{
		ServiceName:     spec.GetName(),
		ServerOwner:     srv.GetPackageName(),
		EndpointOwner:   srv.GetPackageName(),
		Type:            t,
		Port:            spec.GetPort(),
		ExportedPort:    spec.GetExportedPort(),
		AllocatedName:   fmt.Sprintf("%s-%s", spec.GetName(), srv.Id),
		ServiceLabel:    spec.GetLabel(),
		ServiceMetadata: spec.Metadata,
		IngressProvider: spec.IngressProvider,
	}

	if len(spec.IngressDomain) > 0 {
		if t != schema.Endpoint_INTERNET_FACING {
			return nil, fnerrors.InternalError("ingress domain is specified in non-ingress endpoint")
		}

		endpoint.IngressSpec = &schema.Endpoint_IngressSpec{
			Domain: spec.IngressDomain,
		}
	}

	if endpoint.ExportedPort == 0 {
		endpoint.ExportedPort = spec.GetPort().GetContainerPort()
	}

	// XXX Rethink this -- i.e. consolidate with InternalEndpoint.
	if spec.Internal {
		endpoint.ServiceMetadata = append(endpoint.ServiceMetadata, &schema.ServiceMetadata{
			Kind: constants.ManualInternalService,
		})
	}

	return endpoint, nil
}
