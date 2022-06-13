// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package runtime

import (
	"context"
	"strings"

	"google.golang.org/protobuf/proto"
	anypb "google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/protos"
	"namespacelabs.dev/foundation/internal/uniquestrings"
	"namespacelabs.dev/foundation/schema"
)

const (
	HttpServiceName    = "http"
	IngressServiceName = "ingress"
	IngressServiceKind = "ingress"

	ManualInternalService = "internal-service"
)

var reservedServiceNames = []string{HttpServiceName, GrpcGatewayServiceName, IngressServiceName}

const LocalIngressPort = 40080

type LanguageRuntimeSupport interface {
	InternalEndpoints(*schema.Environment, *schema.Server, []*schema.Endpoint_Port) ([]*schema.InternalEndpoint, error)
}

var supportByFramework = map[string]LanguageRuntimeSupport{}

// XXX this is not the right place for protocol handling.
func RegisterSupport(fmwk schema.Framework, f LanguageRuntimeSupport) {
	supportByFramework[fmwk.String()] = f
}

func ComputeIngress(ctx context.Context, env *schema.Environment, sch *schema.Stack_Entry, allEndpoints []*schema.Endpoint) ([]DeferredIngress, error) {
	var ingresses []DeferredIngress

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
					Method:      p.Method,
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

		attached, err := AttachDomains(ctx, env, sch, &schema.IngressFragment{
			Name:        endpoint.ServiceName,
			Owner:       endpoint.ServerOwner,
			Endpoint:    endpoint,
			Extension:   extensions,
			HttpPath:    paths,
			GrpcService: grpc,
		}, endpoint.AllocatedName)
		if err != nil {
			return nil, err
		}

		ingresses = append(ingresses, attached...)
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

			attached, err := AttachDomains(ctx, env, sch, &schema.IngressFragment{
				Name:     serverScoped(sch.Server, name),
				Owner:    sch.GetPackageName().String(),
				HttpPath: paths,
			}, name)
			if err != nil {
				return nil, err
			}

			ingresses = append(ingresses, attached...)
		}
	}

	return ingresses, nil
}

func AttachDomains(ctx context.Context, env *schema.Environment, sch *schema.Stack_Entry, fragment *schema.IngressFragment, allocatedName string) ([]DeferredIngress, error) {
	domains, err := computeDomains(env, sch.Server, sch.ServerNaming, allocatedName)
	if err != nil {
		return nil, err
	}

	var ingresses []DeferredIngress
	for _, domain := range domains {
		// It can be modified below.
		fragment := protos.Clone(fragment)

		// XXX security this exposes all services registered at port: #102.
		t := DeferredIngress{
			domain:   domain,
			fragment: fragment,
		}

		ingresses = append(ingresses, t)
	}

	return ingresses, nil
}

type DeferredIngress struct {
	domain   DeferredDomain
	fragment *schema.IngressFragment
}

func (d DeferredIngress) WithoutAllocation() *schema.IngressFragment {
	fragment := protos.Clone(d.fragment)
	fragment.Domain = d.domain.Domain
	return fragment
}

func (d DeferredIngress) Allocate(ctx context.Context) (*schema.IngressFragment, error) {
	domain := d.domain.Domain
	if d.domain.AllocateDomain != nil {
		var err error
		domain, err = d.domain.AllocateDomain(ctx)
		if err != nil {
			return nil, err
		}
	}

	fragment := protos.Clone(d.fragment)
	fragment.Domain = domain
	return fragment, nil
}

type DeferredDomain struct {
	Domain *schema.Domain

	AllocateDomain func(context.Context) (*schema.Domain, error)
}

func computeDomains(env *schema.Environment, srv *schema.Server, naming *schema.Naming, allocatedName string) ([]DeferredDomain, error) {
	var domains []DeferredDomain

	domain, err := GuessAllocatedName(env, srv, naming, allocatedName)
	if err != nil {
		return nil, err
	}

	domains = append(domains, DeferredDomain{
		Domain: domain,
		AllocateDomain: func(ctx context.Context) (*schema.Domain, error) {
			return allocateWildcard(ctx, env, srv, naming, allocatedName)
		},
	})

	for _, d := range naming.GetAdditionalTlsManaged() {
		d := d // Capture d.
		if d.AllocatedName == allocatedName {
			domains = append(domains, DeferredDomain{
				Domain: &schema.Domain{Fqdn: d.Fqdn, Managed: schema.Domain_USER_SPECIFIED_TLS_MANAGED},
				AllocateDomain: func(ctx context.Context) (*schema.Domain, error) {
					return allocateName(ctx, srv, naming, fnapi.AllocateOpts{FQDN: d.Fqdn}, schema.Domain_USER_SPECIFIED_TLS_MANAGED, d.Fqdn+".specific")
				},
			})
		}
	}

	for _, d := range naming.GetAdditionalUserSpecified() {
		if d.AllocatedName == allocatedName {
			domains = append(domains, DeferredDomain{
				Domain: &schema.Domain{Fqdn: d.Fqdn, Managed: schema.Domain_USER_SPECIFIED},
			})
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
				return nil, fnerrors.InternalError("inconsistency in domain definitions -- was: %#v now: %#v", previous.Domain, d)
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
