// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package runtime

import (
	"context"
	"fmt"
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

		attached, err := AttachComputedDomains(ctx, env, sch, &schema.IngressFragment{
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

			attached, err := AttachComputedDomains(ctx, env, sch, &schema.IngressFragment{
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

func AttachComputedDomains(ctx context.Context, env *schema.Environment, sch *schema.Stack_Entry, template *schema.IngressFragment, allocatedName string) ([]*schema.IngressFragment, error) {
	domains, err := computeDomains(env, sch.ServerNaming, allocatedName)
	if err != nil {
		return nil, err
	}

	var ingresses []*schema.IngressFragment
	for _, domain := range domains {
		// It can be modified below.
		fragment := protos.Clone(template)
		fragment.Domain = domain
		ingresses = append(ingresses, fragment)
	}

	return ingresses, nil
}

func MaybeAllocateDomainCertificate(ctx context.Context, entry *schema.Stack_Entry, template *schema.Domain) (*schema.Domain, error) {
	domain := protos.Clone(template)

	switch domain.Managed {
	case schema.Domain_LOCAL_MANAGED:
		// Nothing to do

	case schema.Domain_CLOUD_MANAGED:
		if !strings.HasSuffix(domain.Fqdn, "."+CloudBaseDomain) {
			return nil, fnerrors.InternalError("%s: expected a %q suffix", domain.Fqdn, CloudBaseDomain)
		}

		withoutSuffix := strings.TrimSuffix(domain.Fqdn, "."+CloudBaseDomain)
		parts := strings.Split(withoutSuffix, ".") // Name, Env, Org
		if len(parts) != 3 {
			return nil, fnerrors.InternalError("%s: expected domain to be {name}.{env}.{org}", domain.Fqdn)
		}

		cert, err := allocateName(ctx, entry.Server, entry.ServerNaming, fnapi.AllocateOpts{
			Subdomain: fmt.Sprintf("%s.%s", parts[0], parts[1]),
			Org:       parts[2],
		})
		if err != nil {
			return nil, err
		}
		domain.Certificate = cert

	case schema.Domain_USER_SPECIFIED_TLS_MANAGED:
		cert, err := allocateName(ctx, entry.Server, entry.ServerNaming, fnapi.AllocateOpts{
			FQDN: domain.Fqdn,
		})
		if err != nil {
			return nil, err
		}
		domain.Certificate = cert

	case schema.Domain_USER_SPECIFIED:
		// Nothing to do.
	}

	return domain, nil
}

func computeDomains(env *schema.Environment, naming *schema.Naming, allocatedName string) ([]*schema.Domain, error) {
	computed, err := ComputeNaming(env, naming)
	if err != nil {
		return nil, err
	}

	return CalculateDomains(env, computed, allocatedName)
}

func CalculateDomains(env *schema.Environment, computed *schema.ComputedNaming, allocatedName string) ([]*schema.Domain, error) {
	var domains []*schema.Domain

	domains = append(domains, &schema.Domain{
		// XXX include stack?
		Fqdn:    fmt.Sprintf("%s.%s.%s", allocatedName, env.Name, computed.BaseDomain),
		Managed: computed.Managed,
	})

	naming := computed.Source

	for _, d := range naming.GetAdditionalTlsManaged() {
		d := d // Capture d.
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
