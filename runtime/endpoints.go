// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package runtime

import (
	"fmt"

	"golang.org/x/exp/slices"
	anypb "google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
)

func ComputeEndpoints(env *schema.Environment, sch *schema.Stack_Entry, allocatedPorts []*schema.Endpoint_Port) ([]*schema.Endpoint, []*schema.InternalEndpoint, error) {
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
		nd, err := computeServiceEndpoint(sch.Server, service, service.GetIngress(), serverPort)
		if err != nil {
			return nil, nil, err
		}
		endpoints = append(endpoints, nd...)
	}

	// Handle statically defined services.
	srv := sch.Server
	for _, s := range srv.GetService() {
		spec, err := ServiceSpecToEndpoint(srv, s, schema.Endpoint_PRIVATE)
		if err != nil {
			return nil, nil, err
		}
		endpoints = append(endpoints, spec)
	}

	for _, s := range srv.GetIngress() {
		spec, err := ServiceSpecToEndpoint(srv, s, schema.Endpoint_INTERNET_FACING)
		if err != nil {
			return nil, nil, err
		}
		endpoints = append(endpoints, spec)
	}

	var gatewayServices []string
	var publicGateway bool
	for _, endpoint := range endpoints {
		for _, md := range endpoint.ServiceMetadata {
			if md.Kind == kindNeedsGrpcGateway {
				exported := &schema.GrpcExportService{}
				if err := md.Details.UnmarshalTo(exported); err != nil {
					return nil, nil, err
				}

				gatewayServices = append(gatewayServices, exported.ProtoTypename)

				if endpoint.Type == schema.Endpoint_INTERNET_FACING {
					publicGateway = true
				}
			}
		}
	}

	server := sch.Server

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

	slices.Sort(gatewayServices)
	gatewayServices = slices.Compact(gatewayServices)

	if len(gatewayServices) > 0 {
		var hasTranscodeNode bool

		for _, imp := range sch.Server.Import {
			if GrpcHttpTranscodeNode.Equals(imp) {
				hasTranscodeNode = true
				break
			}
		}

		if !hasTranscodeNode {
			switch server.Framework {
			case schema.Framework_GO_GRPC:
				if UseGoInternalGrpcGateway {
					gwEndpoint, err := makeGrpcGatewayEndpoint(sch.Server, serverPorts, gatewayServices, publicGateway)
					if err != nil {
						return nil, nil, err
					}

					// We need a http service to hit.
					endpoints = append(endpoints, gwEndpoint)
				}

			default:
				return nil, nil, fnerrors.New("server includes grpc gateway requirements, but it's not of a supported framework")
			}
		}
	}

	var internal []*schema.InternalEndpoint

	if f, ok := supportByFramework[server.Framework.String()]; ok {
		var err error
		internal, err = f.InternalEndpoints(env, server, allocatedPorts)
		if err != nil {
			return nil, nil, err
		}
	}

	return endpoints, internal, nil
}

// XXX this should be somewhere else.
func computeServiceEndpoint(server *schema.Server, n *schema.Node, t schema.Endpoint_Type, serverPort *schema.Endpoint_Port) ([]*schema.Endpoint, error) {
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
			Protocol: schema.GrpcProtocol,
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
		}
	}

	return []*schema.Endpoint{endpoint}, nil
}

func ServiceSpecToEndpoint(srv *schema.Server, spec *schema.Server_ServiceSpec, t schema.Endpoint_Type) (*schema.Endpoint, error) {
	endpoint := &schema.Endpoint{
		ServiceName:   spec.GetName(),
		ServerOwner:   srv.GetPackageName(),
		EndpointOwner: srv.GetPackageName(),
		Type:          t,
		Port:          spec.GetPort(),
		AllocatedName: fmt.Sprintf("%s-%s", spec.GetName(), srv.Id),
	}

	if spec.Metadata != nil {
		endpoint.ServiceMetadata = []*schema.ServiceMetadata{spec.Metadata}
	}

	if spec.Internal {
		endpoint.ServiceMetadata = append(endpoint.ServiceMetadata, &schema.ServiceMetadata{
			Kind: ManualInternalService,
		})
	}

	return endpoint, nil
}
