// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package runtime

import (
	"context"
	"fmt"
	"strings"

	"golang.org/x/exp/slices"
	"google.golang.org/protobuf/proto"
	anypb "google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/uniquestrings"
	"namespacelabs.dev/foundation/schema"
)

const (
	// XXX this is not quite right; it's just a simple mechanism for language and runtime
	// to communicate. Ideally the schema would incorporate a gRPC map.
	KindNeedsGrpcGateway = "needs-grpc-gateway"

	HttpServiceName        = "http"
	GrpcGatewayServiceName = "grpc-gateway"
	GrpcGatewayServiceKind = "grpc-gateway"
	IngressServiceName     = "ingress"
	IngressServiceKind     = "ingress"

	ManualInternalService = "internal-service"
)

var reservedServiceNames = []string{HttpServiceName, GrpcGatewayServiceName, IngressServiceName}

const LocalIngressPort = 40080

type LanguageRuntimeSupport interface {
	FillEndpoint(*schema.Node, *schema.Endpoint) error
	InternalEndpoints(*schema.Environment, *schema.Server, []*schema.Endpoint_Port) ([]*schema.InternalEndpoint, error)
}

var supportByFramework = map[string]LanguageRuntimeSupport{}

// XXX this is not the right place for protocol handling.
func RegisterSupport(fmwk schema.Framework, f LanguageRuntimeSupport) {
	supportByFramework[fmwk.String()] = f
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

	if f, ok := supportByFramework[server.Framework.String()]; ok {
		if err := f.FillEndpoint(n, endpoint); err != nil {
			return nil, err
		}
	}

	return []*schema.Endpoint{endpoint}, nil
}

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
			if md.Kind == KindNeedsGrpcGateway {
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

	if len(gatewayServices) > 0 {
		var gwPort *schema.Endpoint_Port
		for _, port := range serverPorts {
			if port.Name == "grpc-gateway-port" {
				gwPort = port
				break
			}
		}

		// This entrypoint is otherwise open to any caller, so follow the same
		// policy for browser-based requests.
		cors := &schema.HttpCors{Enabled: true, AllowedOrigin: []string{"*"}}
		packedCors, err := anypb.New(cors)
		if err != nil {
			return nil, nil, fnerrors.UserError(nil, "failed to pack CORS' configuration: %v", err)
		}

		urlMap := &schema.HttpUrlMap{}
		for _, svc := range gatewayServices {
			urlMap.Entry = append(urlMap.Entry, &schema.HttpUrlMap_Entry{
				PathPrefix: fmt.Sprintf("/%s/", svc),
				Kind:       GrpcGatewayServiceKind,
			})
		}
		packedUrlMap, err := anypb.New(urlMap)
		if err != nil {
			return nil, nil, fnerrors.InternalError("failed to marshal url map: %w", err)
		}

		gwEndpoint := &schema.Endpoint{
			Type:          schema.Endpoint_PRIVATE,
			ServiceName:   GrpcGatewayServiceName,
			Port:          gwPort,
			AllocatedName: grpcGatewayName(sch.Server),
			EndpointOwner: sch.Server.GetPackageName(),
			ServerOwner:   sch.Server.GetPackageName(),
			ServiceMetadata: []*schema.ServiceMetadata{
				{Protocol: "http", Details: packedUrlMap},
				{Protocol: "http", Kind: "http-extension", Details: packedCors},
			},
		}

		if publicGateway {
			gwEndpoint.Type = schema.Endpoint_INTERNET_FACING
		}

		// We need a http service to hit.
		endpoints = append(endpoints, gwEndpoint)
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

func grpcGatewayName(srv *schema.Server) string {
	return GrpcGatewayServiceName + "-" + srv.Id
}

func ComputeIngress(ctx context.Context, env *schema.Environment, sch *schema.Stack_Entry, allEndpoints []*schema.Endpoint) ([]*schema.IngressFragment, error) {
	var ingresses []*schema.IngressFragment

	var serverEndpoints []*schema.Endpoint
	for _, endpoint := range allEndpoints {
		if endpoint.ServerOwner == sch.Server.PackageName {
			serverEndpoints = append(serverEndpoints, endpoint)
		}
	}

	for _, endpoint := range serverEndpoints {
		if !(endpoint.Type == schema.Endpoint_INTERNET_FACING && endpoint.Port != nil) {
			continue
		}

		var protocol *string
		var protocolDetails []*anypb.Any
		var extensions []*anypb.Any
		for _, md := range endpoint.ServiceMetadata {
			if md.Protocol != "" {
				if protocol == nil {
					protocol = &md.Protocol
					if md.Details != nil {
						protocolDetails = append(protocolDetails, md.Details)
					}
				} else if *protocol != md.Protocol {
					return nil, fnerrors.InternalError("%s: inconsistent protocol definition, %q and %q", endpoint.GetServiceName(), *protocol, md.Protocol)
				}
			}

			if md.Kind == "http-extension" && md.Details != nil {
				extensions = append(extensions, md.Details)
			}
		}

		if protocol == nil {
			continue
		}

		var kind string
		if *protocol != "http" {
			kind = *protocol
		}

		var paths []*schema.IngressFragment_IngressHttpPath
		var grpc []*schema.IngressFragment_IngressGrpcService

		switch *protocol {
		case "http":
			for _, details := range protocolDetails {
				p := &schema.HttpUrlMap{}
				if err := details.UnmarshalTo(p); err != nil {
					return nil, err
				}
				for _, entry := range p.Entry {
					paths = append(paths, &schema.IngressFragment_IngressHttpPath{
						Path:    entry.PathPrefix,
						Kind:    kind,
						Owner:   endpoint.EndpointOwner,
						Service: endpoint.AllocatedName,
						Port:    endpoint.Port,
					})
				}
			}

			// XXX still relevant? We used to do this when grpc followed the http path.
			if len(paths) == 0 {
				paths = []*schema.IngressFragment_IngressHttpPath{
					{Path: "/", Kind: kind, Owner: endpoint.EndpointOwner, Service: endpoint.AllocatedName, Port: endpoint.Port},
				}
			}

		case "grpc":
			for _, details := range protocolDetails {
				p := &schema.GrpcExportService{}
				if err := details.UnmarshalTo(p); err != nil {
					return nil, err
				}
				grpc = append(grpc, &schema.IngressFragment_IngressGrpcService{
					GrpcService: p.ProtoTypename,
					Owner:       endpoint.EndpointOwner,
					Service:     endpoint.AllocatedName,
					Port:        endpoint.Port,
				})
				// XXX rethink this.
				grpc = append(grpc, &schema.IngressFragment_IngressGrpcService{
					GrpcService: "grpc.reflection.v1alpha.ServerReflection",
					Owner:       endpoint.EndpointOwner,
					Service:     endpoint.AllocatedName,
					Port:        endpoint.Port,
				})
			}
		}

		domains, err := makeDomains(ctx, env, sch.Server, sch.ServerNaming, endpoint.AllocatedName)
		if err != nil {
			return nil, err
		}

		for _, domain := range domains {
			// XXX security this exposes all services registered at port: #102.
			t := &schema.IngressFragment{
				Domain:      domain,
				Name:        endpoint.ServiceName,
				Owner:       endpoint.ServerOwner,
				Endpoint:    endpoint,
				Extension:   extensions,
				HttpPath:    paths,
				GrpcService: grpc,
			}

			if t.Domain.Managed == schema.Domain_CLOUD_MANAGED || t.Domain.Managed == schema.Domain_LOCAL_MANAGED {
				t.Name += "-managed"
			}

			ingresses = append(ingresses, t)
		}
	}

	// Handle HTTP.
	if needsHTTP := len(sch.Server.UrlMap) > 0; needsHTTP {
		var httpEndpoint *schema.Endpoint
		for _, endpoint := range serverEndpoints {
			if endpoint.ServiceName == "http" {
				httpEndpoint = endpoint
				break
			}
		}

		if httpEndpoint == nil {
			return nil, fnerrors.InternalError("urlmap is present, but http endpoint is missing")
		}

		perIngress := map[string][]*schema.Server_URLMapEntry{}
		ingressNames := uniquestrings.List{}

		for _, u := range sch.Server.UrlMap {
			ingressName := u.IngressName
			if ingressName == "" {
				ingressName = httpEndpoint.AllocatedName
			}

			perIngress[ingressName] = append(perIngress[ingressName], u)
			ingressNames.Add(ingressName)
		}

		for _, name := range ingressNames.Strings() {
			var paths []*schema.IngressFragment_IngressHttpPath

			for _, u := range perIngress[name] {
				owner := u.PackageName
				if owner == "" {
					owner = sch.Server.PackageName
				}

				paths = append(paths, &schema.IngressFragment_IngressHttpPath{
					Path:    u.PathPrefix,
					Kind:    u.Kind,
					Owner:   owner,
					Service: httpEndpoint.AllocatedName,
					Port:    httpEndpoint.Port,
				})
			}

			domains, err := makeDomains(ctx, env, sch.Server, sch.ServerNaming, name)
			if err != nil {
				return nil, err
			}

			for _, domain := range domains {
				t := &schema.IngressFragment{
					Domain:   domain,
					Name:     serverScoped(sch.Server, name),
					Owner:    sch.GetPackageName().String(),
					HttpPath: paths,
				}

				if t.Domain.Managed == schema.Domain_CLOUD_MANAGED || t.Domain.Managed == schema.Domain_LOCAL_MANAGED {
					t.Name += "-managed"
				}

				ingresses = append(ingresses, t)
			}
		}
	}

	return ingresses, nil
}

func makeDomains(ctx context.Context, env *schema.Environment, srv *schema.Server, naming *schema.Naming, allocatedName string) ([]*schema.Domain, error) {
	// XXX pass in auth.
	allocated, err := allocateWildcard(ctx, env, srv, naming, allocatedName)
	if err != nil {
		return nil, err
	}

	var domains []*schema.Domain
	if allocated.GetFqdn() != "" {
		domains = append(domains, allocated)
	}

	for _, d := range naming.GetAdditionalTlsManaged() {
		if d.AllocatedName == allocatedName {
			domain, err := allocateName(ctx, srv, naming, fnapi.AllocateOpts{FQDN: d.Fqdn}, schema.Domain_USER_SPECIFIED_TLS_MANAGED, d.Fqdn+".specific")
			if err != nil {
				return nil, err
			}

			domains = append(domains, domain)
		}
	}

	for _, d := range naming.GetAdditionalUserSpecified() {
		if d.AllocatedName == allocatedName {
			domains = append(domains, &schema.Domain{Fqdn: d.Fqdn, Managed: schema.Domain_USER_SPECIFIED})
		}
	}

	return domains, nil
}

func GuessDomains(env *schema.Environment, srv *schema.Server, naming *schema.Naming, allocatedName string) ([]*schema.Domain, error) {
	var domains []*schema.Domain
	d, err := GuessAllocatedName(env, srv, naming, allocatedName)
	if err != nil {
		return nil, err
	}

	domains = append(domains, d)

	for _, d := range naming.GetAdditionalTlsManaged() {
		if d.AllocatedName == allocatedName {
			domains = append(domains, &schema.Domain{Fqdn: d.Fqdn, Managed: schema.Domain_USER_SPECIFIED_TLS_MANAGED})

		}
	}

	for _, d := range naming.GetAdditionalUserSpecified() {
		if d.AllocatedName == allocatedName {
			domains = append(domains, &schema.Domain{Fqdn: d.Fqdn, Managed: schema.Domain_USER_SPECIFIED})
		}
	}

	return domains, nil
}

func serverScoped(srv *schema.Server, name string) string {
	name = srv.Name + "-" + name

	if !strings.HasSuffix(name, "-"+srv.Id) {
		return name + "-" + srv.Id
	}

	return name
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

type FilteredDomain struct {
	Domain    *schema.Domain
	Endpoints []*schema.Endpoint
}

func FilterAndDedupDomains(fragments []*schema.IngressFragment, filter func(*schema.Domain) bool) ([]*FilteredDomain, error) {
	seenFQDN := map[string]*FilteredDomain{} // Map fqdn to schema.
	domains := []*FilteredDomain{}
	for _, frag := range fragments {
		d := frag.Domain

		if d.GetFqdn() == "" {
			continue
		}

		if filter != nil && !filter(d) {
			continue
		}

		if previous, ok := seenFQDN[d.Fqdn]; ok {
			if !proto.Equal(previous.Domain, d) {
				return nil, fnerrors.InternalError("%s: inconsistency in domain definitions", d.Fqdn)
			}
		} else {
			fd := &FilteredDomain{Domain: d}
			domains = append(domains, fd)
			seenFQDN[d.Fqdn] = fd
		}

		if frag.Endpoint != nil {
			seenFQDN[d.Fqdn].Endpoints = append(seenFQDN[d.Fqdn].Endpoints, frag.Endpoint)
		}
	}

	return domains, nil
}
