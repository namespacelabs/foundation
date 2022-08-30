// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package runtime

import (
	"fmt"

	"golang.org/x/exp/slices"
	"google.golang.org/protobuf/types/descriptorpb"
	anypb "google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
)

func ComputeEndpoints(srv provision.Server, allocatedPorts []*schema.Endpoint_Port) ([]*schema.Endpoint, []*schema.InternalEndpoint, error) {
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
		var pkg *workspace.Package
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
	for _, s := range server.GetService() {
		spec, err := ServiceSpecToEndpoint(server, s, schema.Endpoint_PRIVATE)
		if err != nil {
			return nil, nil, err
		}
		endpoints = append(endpoints, spec)
	}

	for _, s := range server.GetIngress() {
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
			ServiceName:   HttpServiceName,
			Port:          httpPort,
			AllocatedName: server.Name,
			EndpointOwner: server.GetPackageName(),
			ServerOwner:   server.GetPackageName(),
			ServiceMetadata: []*schema.ServiceMetadata{
				{Protocol: "http"},
			},
		})
	}

	var internal []*schema.InternalEndpoint

	if f, ok := supportByFramework[server.Framework.String()]; ok {
		var err error
		internal, err = f.InternalEndpoints(srv.Env().Proto(), server, allocatedPorts)
		if err != nil {
			return nil, nil, err
		}
	}

	return endpoints, internal, nil
}

// XXX this should be somewhere else.
func computeServiceEndpoint(server *schema.Server, pkg *workspace.Package, n *schema.Node, t schema.Endpoint_Type, serverPort *schema.Endpoint_Port) ([]*schema.Endpoint, error) {
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
	}

	if slices.Contains(reservedServiceNames, endpoint.ServiceName) {
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
				Kind:    kindNeedsGrpcGateway,
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
		AllocatedName:   fmt.Sprintf("%s-%s", spec.GetName(), srv.Id),
		ServiceLabel:    spec.GetLabel(),
		ServiceMetadata: spec.Metadata,
	}

	// XXX Rethink this -- i.e. consolidate with InternalEndpoint.
	if spec.Internal {
		endpoint.ServiceMetadata = append(endpoint.ServiceMetadata, &schema.ServiceMetadata{
			Kind: ManualInternalService,
		})
	}

	return endpoint, nil
}
