// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package runtime

import (
	"context"
	"fmt"
	"strings"

	anypb "google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/framework/kubernetes/kubenaming"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/planning/constants"
	"namespacelabs.dev/foundation/internal/protos"
	"namespacelabs.dev/foundation/internal/uniquestrings"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
)

const LocalIngressPort = 40080

func ComputeIngress(ctx context.Context, env cfg.Context, planner Planner, sch *schema.Stack_Entry, allEndpoints []*schema.Endpoint) ([]*schema.IngressFragment, error) {
	var ingresses []*schema.IngressFragment

	var serverEndpoints []*schema.Endpoint
	for _, endpoint := range allEndpoints {
		if endpoint.ServerOwner == sch.Server.PackageName {
			serverEndpoints = append(serverEndpoints, endpoint)
		}
	}

	for _, endpoint := range serverEndpoints {
		if !(endpoint.Type == schema.Endpoint_INTERNET_FACING && len(endpoint.Ports) > 0) {
			continue
		}

		pm := endpoint.Ports[0]

		if endpoint.IngressProvider != nil {
			fmt.Fprintf(console.Debug(ctx), "Skipping endpoint %s/%s: has ingress provider\n", endpoint.EndpointOwner, endpoint.AllocatedName)
			continue
		}

		var protocol *string
		var protocolDetails []*anypb.Any
		var httpExtensions []*anypb.Any
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
				httpExtensions = append(httpExtensions, md.Details)
			}
		}

		if protocol == nil {
			fmt.Fprintf(console.Debug(ctx), "Skipping endpoint %s/%s: no protocol\n", endpoint.EndpointOwner, endpoint.AllocatedName)
			continue
		}

		var paths []*schema.IngressFragment_IngressHttpPath
		var grpc []*schema.IngressFragment_IngressGrpcService
		var clearTextGrpcCount int

		switch *protocol {
		case schema.HttpProtocol:
			var kind string
			if *protocol != schema.HttpProtocol {
				kind = *protocol
			}

			for _, details := range protocolDetails {
				p := &schema.HttpUrlMap{}
				if err := details.UnmarshalTo(p); err != nil {
					return nil, err
				}

				for _, entry := range p.Entry {
					target := endpoint
					if entry.BackendService != nil {
						for _, nd := range allEndpoints {
							if nd.EndpointOwner == entry.BackendService.PackageName && nd.ServiceName == entry.BackendService.Name {
								target = nd
							}
						}
					}

					paths = append(paths, &schema.IngressFragment_IngressHttpPath{
						Path:        entry.PathPrefix,
						Kind:        kind,
						Owner:       target.EndpointOwner,
						Service:     target.AllocatedName,
						ServicePort: pm.ExportedPort,
					})
				}
			}

			// XXX still relevant? We used to do this when grpc followed the http path.
			if len(paths) == 0 {
				paths = []*schema.IngressFragment_IngressHttpPath{
					{Path: "/", Kind: kind, Owner: endpoint.EndpointOwner,
						Service:     endpoint.AllocatedName,
						ServicePort: pm.ExportedPort},
				}
			}

		case schema.GrpcProtocol, schema.ClearTextGrpcProtocol:
			for _, details := range protocolDetails {
				msg, err := details.UnmarshalNew()
				if err != nil {
					return nil, fnerrors.InternalError("failed to unserialize grpc configuration: %w", err)
				}

				if *protocol == schema.ClearTextGrpcProtocol {
					clearTextGrpcCount++
				}

				switch p := msg.(type) {
				case *schema.GrpcExportService:
					grpc = append(grpc, &schema.IngressFragment_IngressGrpcService{
						GrpcService: p.ProtoTypename,
						Owner:       endpoint.EndpointOwner,
						Service:     endpoint.AllocatedName,
						ServicePort: pm.ExportedPort,
						Method:      p.Method,
						BackendTls:  *protocol == schema.GrpcProtocol,
					})

					if p.ServerReflectionIncluded {
						grpc = append(grpc, &schema.IngressFragment_IngressGrpcService{
							GrpcService: "grpc.reflection.v1alpha.ServerReflection",
							Owner:       endpoint.EndpointOwner,
							Service:     endpoint.AllocatedName,
							ServicePort: pm.ExportedPort,
							BackendTls:  *protocol == schema.GrpcProtocol,
						})
					}

				case *schema.GrpcExportAllServices:
					grpc = append(grpc, &schema.IngressFragment_IngressGrpcService{
						AllServices: true,
						Owner:       endpoint.EndpointOwner,
						Service:     endpoint.AllocatedName,
						ServicePort: pm.ExportedPort,
						BackendTls:  *protocol == schema.GrpcProtocol,
					})

				default:
					return nil, fnerrors.InternalError("unsupported grpc configuration: %v", p.ProtoReflect().Descriptor().FullName())
				}
			}

		default:
			return nil, fnerrors.Newf("%s: unsupported ingress protocol", *protocol)
		}

		if len(paths) > 0 && len(grpc) > 0 {
			return nil, fnerrors.BadInputError("can't mix grpc and http paths within a single endpoint")
		}

		attached, err := AttachComputedDomains(ctx, env.Workspace().ModuleName(), env, planner, sch, &schema.IngressFragment{
			Name:        endpoint.ServiceName,
			Owner:       endpoint.ServerOwner,
			Endpoint:    endpoint,
			Extension:   httpExtensions,
			HttpPath:    paths,
			GrpcService: grpc,
		}, DomainsRequest{
			ServerID:      sch.Server.Id,
			ServiceName:   endpoint.ServiceName,
			Key:           endpoint.AllocatedName,
			Alias:         endpoint.ServiceName,
			RequiresTLS:   clearTextGrpcCount > 0,
			UserSpecified: endpoint.GetIngressSpec().GetDomain(),
		})
		if err != nil {
			return nil, err
		}

		ingresses = append(ingresses, attached...)
	}

	// Handle HTTP.
	if needsHTTP := len(sch.Server.UrlMap) > 0; needsHTTP {
		var httpEndpoints []*schema.Endpoint
		for _, endpoint := range serverEndpoints {
			if endpoint.ServiceName == constants.HttpServiceName && len(endpoint.Ports) > 0 {
				httpEndpoints = append(httpEndpoints, endpoint)
				break
			}
		}

		if len(httpEndpoints) != 1 {
			return nil, fnerrors.InternalError("urlmap is present, but single http endpoint is missing")
		}

		httpEndpoint := httpEndpoints[0]

		perIngress := map[string][]*schema.Server_URLMapEntry{}
		perIngressAlias := map[string]string{}
		ingressNames := uniquestrings.List{}

		for _, url := range sch.Server.UrlMap {
			if !url.Public {
				continue
			}

			ingressName := url.IngressName
			alias := url.IngressName
			if ingressName == "" {
				ingressName = httpEndpoint.AllocatedName
				alias = httpEndpoint.ServiceName
			}

			perIngress[ingressName] = append(perIngress[ingressName], url)
			perIngressAlias[ingressName] = alias
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
					Path:            u.PathPrefix,
					Kind:            u.Kind,
					Owner:           owner,
					Service:         httpEndpoint.AllocatedName,
					ServicePort:     httpEndpoint.Ports[0].ExportedPort,
					BackendProtocol: u.BackendProtocol,
				})
			}

			attached, err := AttachComputedDomains(ctx, env.Workspace().ModuleName(), env, planner, sch, &schema.IngressFragment{
				Name:     serverScoped(sch.Server, name),
				Owner:    sch.GetPackageName().String(),
				HttpPath: paths,
			}, DomainsRequest{
				ServerID: sch.GetServer().GetId(),
				Key:      name,
				Alias:    perIngressAlias[name],
			})
			if err != nil {
				return nil, err
			}

			ingresses = append(ingresses, attached...)
		}
	}

	return ingresses, nil
}

func AttachComputedDomains(ctx context.Context, ws string, env cfg.Context, cluster Planner, sch *schema.Stack_Entry, template *schema.IngressFragment, allocatedName DomainsRequest) ([]*schema.IngressFragment, error) {
	domains, err := computeDomains(ctx, ws, env, cluster, sch.ServerNaming, allocatedName)
	if err != nil {
		return nil, err
	}

	fmt.Fprintf(console.Debug(ctx), "%s: %s: computed domains: %v (grpc_services: %v)\n", sch.Server.PackageName, template.GetName(), domains, template.GrpcService)

	var ingresses []*schema.IngressFragment
	for _, domain := range domains {
		// It can be modified below.
		fragment := protos.Clone(template)
		fragment.Domain = domain
		ingresses = append(ingresses, fragment)
	}

	return ingresses, nil
}

func computeDomains(ctx context.Context, ws string, env cfg.Context, cluster Planner, serverNaming *schema.Naming, allocatedName DomainsRequest) ([]*schema.Domain, error) {
	computed, err := ComputeNaming(ctx, ws, env, cluster, serverNaming)
	if err != nil {
		return nil, err
	}

	return CalculateDomains(env.Environment(), computed, allocatedName)
}

type DomainsRequest struct {
	ServerID    string
	ServiceName string
	Key         string // Usually `{ServiceName}-{ServerID}`
	Alias       string

	// Set to true if the service we're allocating a domain for should be TLS
	// terminated, regardless of whether we can emit a public-CA rooted
	// certificate or not. E.g. for gRPC.
	RequiresTLS bool

	UserSpecified []*schema.DomainSpec
}

func CalculateDomains(env *schema.Environment, computed *schema.ComputedNaming, allocatedName DomainsRequest) ([]*schema.Domain, error) {
	var domains []*schema.Domain

	if computed.GetManaged() == schema.Domain_CLOUD_MANAGED ||
		computed.GetManaged() == schema.Domain_CLOUD_TERMINATION ||
		computed.GetManaged() == schema.Domain_LOCAL_MANAGED {
		inclusterTls := allocatedName.RequiresTLS || env.Purpose == schema.Environment_PRODUCTION

		computedDomain := &schema.Domain{
			Managed: computed.Managed,
			// If we have TLS termination at an upstream ingress (e.g. in nscloud's
			// ingress), then still emit https (etc) addresses regardless of whether
			// the in-cluster ingress has TLS termination or not.
			TlsFrontend: computed.UpstreamTlsTermination || inclusterTls,
		}

		if computed.UseShortAlias {
			// grpc-abcdef.nslocal.host
			//
			// grpc-abcdef.hugosantos.nscloud.dev
			//
			// grpc-abcdef-9d5h25dto9nkm.a.nscluster.cloud
			// -> abcdef = sha256(env.name, pkggraph.Module_name)[6:]

			if computed.MainModuleName == "" {
				return nil, fnerrors.NamespaceTooOld("domain allocation", 0, 0)
			}

			x := kubenaming.StableIDN(fmt.Sprintf("%s:%s", env.Name, computed.MainModuleName), 6)
			name := fmt.Sprintf("%s-%s", allocatedName.Alias, x)

			if computed.DomainFragmentSuffix != "" {
				computedDomain.Fqdn = fmt.Sprintf("%s-%s", name, computed.DomainFragmentSuffix)
			} else {
				computedDomain.Fqdn = name
			}
		} else {
			if computed.DomainFragmentSuffix != "" {
				computedDomain.Fqdn = fmt.Sprintf("%s-%s-%s", allocatedName.Key, env.Name, computed.DomainFragmentSuffix)
			} else {
				computedDomain.Fqdn = fmt.Sprintf("%s.%s", allocatedName.Key, env.Name)
			}
		}

		baseDomain := computed.BaseDomain
		// XXX make these runtime calls?
		if allocatedName.RequiresTLS && computed.TlsPassthroughBaseDomain != "" {
			baseDomain = computed.TlsPassthroughBaseDomain
		}

		computedDomain.Fqdn += "." + baseDomain

		domains = append(domains, computedDomain)
	}

	for _, d := range computed.GetSource().GetAdditionalTlsManaged() {
		d := d // Capture d.
		if d.AllocatedName == allocatedName.Key {
			domains = append(domains, &schema.Domain{
				Fqdn:        d.Fqdn,
				Managed:     schema.Domain_USER_SPECIFIED_TLS_MANAGED,
				TlsFrontend: true,
			})
		}
	}

	for _, d := range computed.GetSource().GetAdditionalUserSpecified() {
		if d.AllocatedName == allocatedName.Key {
			domains = append(domains, &schema.Domain{
				Fqdn:        d.Fqdn,
				Managed:     schema.Domain_USER_SPECIFIED,
				TlsFrontend: true,
			})
		}
	}

	for _, d := range allocatedName.UserSpecified {
		domain := &schema.Domain{
			Fqdn:    d.Fqdn,
			Managed: d.Managed,
		}

		if d.Managed == schema.Domain_USER_SPECIFIED_TLS_MANAGED {
			domain.TlsFrontend = true
		}

		domains = append(domains, domain)
	}

	return domains, nil
}

func serverScoped(srv Deployable, name string) string {
	name = srv.GetName() + "-" + name

	if !strings.HasSuffix(name, "-"+srv.GetId()) {
		return name + "-" + srv.GetId()
	}

	return name
}
