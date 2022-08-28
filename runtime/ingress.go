// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package runtime

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"

	anypb "google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/protos"
	"namespacelabs.dev/foundation/internal/uniquestrings"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/go-ids"
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

func ComputeIngress(ctx context.Context, env ops.Environment, sch *schema.Stack_Entry, allEndpoints []*schema.Endpoint) ([]*schema.IngressFragment, error) {
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

				// XXX security rethink this.
				grpc = append(grpc, &schema.IngressFragment_IngressGrpcService{
					GrpcService: "grpc.reflection.v1alpha.ServerReflection",
					Owner:       endpoint.EndpointOwner,
					Service:     endpoint.AllocatedName,
					Port:        endpoint.Port,
				})
			}
		}

		attached, err := AttachComputedDomains(ctx, env.Workspace().ModuleName, env, sch, &schema.IngressFragment{
			Name:        endpoint.ServiceName,
			Owner:       endpoint.ServerOwner,
			Endpoint:    endpoint,
			Extension:   httpExtensions,
			HttpPath:    paths,
			GrpcService: grpc,
		}, DomainsRequest{
			ServerID:    sch.Server.Id,
			ServiceName: endpoint.ServiceName,
			Key:         endpoint.AllocatedName,
			Alias:       endpoint.ServiceName,
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
			if endpoint.ServiceName == HttpServiceName {
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
					Path:    u.PathPrefix,
					Kind:    u.Kind,
					Owner:   owner,
					Service: httpEndpoint.AllocatedName,
					Port:    httpEndpoint.Port,
				})
			}

			attached, err := AttachComputedDomains(ctx, env.Workspace().ModuleName, env, sch, &schema.IngressFragment{
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

func AttachComputedDomains(ctx context.Context, ws string, env ops.Environment, sch *schema.Stack_Entry, template *schema.IngressFragment, allocatedName DomainsRequest) ([]*schema.IngressFragment, error) {
	domains, err := computeDomains(ctx, ws, env, sch.ServerNaming, allocatedName)
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

	if domain.TlsInclusterTermination {
		cert, err := allocateName(ctx, entry.Server, fnapi.AllocateOpts{
			FQDN: domain.Fqdn,
			// XXX remove org -- it should be parsed from fqdn.
			Org: entry.ServerNaming.GetWithOrg(),
		})
		if err != nil {
			return nil, err
		}
		domain.Certificate = cert
	}

	return domain, nil
}

func computeDomains(ctx context.Context, ws string, env ops.Environment, naming *schema.Naming, allocatedName DomainsRequest) ([]*schema.Domain, error) {
	computed, err := ComputeNaming(ctx, ws, env, naming)
	if err != nil {
		return nil, err
	}

	return CalculateDomains(env.Proto(), computed, allocatedName)
}

type DomainsRequest struct {
	ServerID    string
	ServiceName string
	Key         string // Usually `{ServiceName}-{ServerID}`
	Alias       string
}

func CalculateDomains(env *schema.Environment, computed *schema.ComputedNaming, allocatedName DomainsRequest) ([]*schema.Domain, error) {
	computedDomain := &schema.Domain{
		Managed:                 computed.Managed,
		TlsFrontend:             computed.TlsFrontend,
		TlsInclusterTermination: computed.TlsInclusterTermination,
	}

	if computed.UseShortAlias {
		// grpc-abcdef.nslocal.host
		//
		// grpc-abcdef.hugosantos.nscloud.dev
		//
		// grpc-abcdef-9d5h25dto9nkm.a.nscluster.cloud
		// -> abcdef = hash(dev, workspace.module_name)

		if computed.MainModuleName == "" {
			return nil, fnerrors.DoesNotMeetVersionRequirements("domain allocation", 0, 0)
		}

		h := sha256.New()
		fmt.Fprintf(h, "%s:%s", env.Name, computed.MainModuleName)
		x := ids.EncodeToBase32String(h.Sum(nil))[:6]
		name := fmt.Sprintf("%s-%s", allocatedName.Alias, x)

		if computed.DomainFragmentSuffix != "" {
			computedDomain.Fqdn = fmt.Sprintf("%s-%s", name, computed.DomainFragmentSuffix)
		} else {
			computedDomain.Fqdn = name
		}
	} else {
		if computed.DomainFragmentSuffix != "" {
			computedDomain.Fqdn = fmt.Sprintf("%s-%s-%s", allocatedName, env.Name, computed.DomainFragmentSuffix)
		} else {
			computedDomain.Fqdn = fmt.Sprintf("%s.%s", allocatedName, env.Name)
		}
	}

	computedDomain.Fqdn += "." + computed.BaseDomain

	domains := []*schema.Domain{computedDomain}

	naming := computed.Source

	for _, d := range naming.GetAdditionalTlsManaged() {
		d := d // Capture d.
		if d.AllocatedName == allocatedName.Key {
			domains = append(domains, &schema.Domain{
				Fqdn:                    d.Fqdn,
				Managed:                 schema.Domain_USER_SPECIFIED_TLS_MANAGED,
				TlsFrontend:             true,
				TlsInclusterTermination: true,
			})
		}
	}

	for _, d := range naming.GetAdditionalUserSpecified() {
		if d.AllocatedName == allocatedName.Key {
			domains = append(domains, &schema.Domain{
				Fqdn:                    d.Fqdn,
				Managed:                 schema.Domain_USER_SPECIFIED,
				TlsFrontend:             true,
				TlsInclusterTermination: false,
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
