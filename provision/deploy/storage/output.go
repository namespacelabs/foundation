// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package storage

import (
	"fmt"
	"strings"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/uniquestrings"
	"namespacelabs.dev/foundation/provision/deploy/render"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/schema/storage"
)

type PortFwd struct {
	Endpoint  *schema.Endpoint
	LocalPort uint32
}

func ToStorageNetworkPlan(localHostname string, stack *schema.Stack, focus []*schema.Server, portFwds []*PortFwd, ingressFragments []*schema.IngressFragment) (*storage.NetworkPlan, error) {
	r := &storage.NetworkPlan{
		LocalHostname: localHostname,
	}

	for _, s := range focus {
		r.FocusedServerPackages = append(r.FocusedServerPackages, s.PackageName)
	}

	for _, i := range ingressFragments {
		domain, err := convertDomain(i.Domain)
		if err != nil {
			return nil, err
		}
		endpoint, err := convertEndpoint(i.Endpoint, 0 /* localPort */, stack)
		if err != nil {
			return nil, err
		}
		fragment := &storage.IngressFragment{
			Name:     i.Name,
			Owner:    i.Owner,
			Domain:   domain,
			Endpoint: endpoint,
			Manager:  i.Manager,
		}
		for _, httpPath := range i.HttpPath {
			fragment.HttpPath = append(fragment.HttpPath, convertHttpPath(httpPath))
		}
		for _, grpcService := range i.GrpcService {
			fragment.GrpcService = append(fragment.GrpcService, convertGrpcService(grpcService))
		}
		r.IngressFragments = append(r.IngressFragments, fragment)
	}

	for _, pfwd := range portFwds {
		endpoint, err := convertEndpoint(pfwd.Endpoint, pfwd.LocalPort, stack)
		if err != nil {
			return nil, err
		}
		r.Endpoints = append(r.Endpoints, endpoint)
	}

	// TODO: remove once "internal" is migrated to "networkplanutils"
	addDeprecatedFields(r)

	return r, nil
}

func convertDomain(d *schema.Domain) (*storage.Domain, error) {
	if d == nil {
		return nil, nil
	}
	managedType, err := convertManagedType(d.Managed)
	if err != nil {
		return nil, err
	}
	return &storage.Domain{
		Fqdn:           d.Fqdn,
		Managed:        managedType,
		HasCertificate: d.Certificate != nil,
	}, nil
}

func convertGrpcService(s *schema.IngressFragment_IngressGrpcService) *storage.IngressGrpcService {
	return &storage.IngressGrpcService{
		GrpcService: s.GrpcService,
		Owner:       s.Owner,
		Service:     s.Service,
		Method:      s.Method,
		Port:        convertPort(s.Port),
	}
}

func convertEndpoint(endpoint *schema.Endpoint, localPort uint32, stack *schema.Stack) (*storage.Endpoint, error) {
	if endpoint == nil {
		return nil, nil
	}

	entry := stack.GetServer(schema.PackageName(endpoint.EndpointOwner))
	serverName := ""
	if entry != nil {
		serverName = entry.Server.Name
	}

	endpointType, err := convertEndpointType(endpoint.Type)
	if err != nil {
		return nil, err
	}
	result := &storage.Endpoint{
		Type:          endpointType,
		ServiceName:   endpoint.ServiceName,
		EndpointOwner: endpoint.EndpointOwner,
		Port:          convertPort(endpoint.Port),
		AllocatedName: endpoint.AllocatedName,
		ServerOwner:   endpoint.ServerOwner,
		ServiceLabel:  endpoint.ServiceLabel,
		LocalPort:     localPort,
		ServerName:    serverName,
	}
	for _, m := range endpoint.ServiceMetadata {
		result.ServiceMetadata = append(result.ServiceMetadata, &storage.Endpoint_ServiceMetadata{
			Kind:     m.Kind,
			Protocol: m.Protocol,
		})
	}

	return result, nil
}

func convertPort(port *schema.Endpoint_Port) *storage.Endpoint_Port {
	if port == nil {
		return nil
	} else {
		return &storage.Endpoint_Port{Name: port.Name, ContainerPort: port.ContainerPort}
	}
}

func convertEndpointType(t schema.Endpoint_Type) (storage.Endpoint_Type, error) {
	switch t {
	case schema.Endpoint_INGRESS_UNSPECIFIED:
		return storage.Endpoint_INGRESS_UNSPECIFIED, nil
	case schema.Endpoint_PRIVATE:
		return storage.Endpoint_PRIVATE, nil
	case schema.Endpoint_INTERNET_FACING:
		return storage.Endpoint_INTERNET_FACING, nil
	default:
		return storage.Endpoint_INGRESS_UNSPECIFIED, fnerrors.InternalError("unknown endpoint type: %s", t)
	}
}

func convertHttpPath(httpPath *schema.IngressFragment_IngressHttpPath) *storage.IngressHttpPath {
	return &storage.IngressHttpPath{
		Path:    httpPath.Path,
		Kind:    httpPath.Kind,
		Owner:   httpPath.Owner,
		Service: httpPath.Service,
		Port:    convertPort(httpPath.Port),
	}
}

func convertManagedType(managed schema.Domain_ManagedType) (storage.Domain_ManagedType, error) {
	switch managed {
	case schema.Domain_MANAGED_UNKNOWN:
		return storage.Domain_MANAGED_UNKNOWN, nil
	case schema.Domain_LOCAL_MANAGED:
		return storage.Domain_LOCAL_MANAGED, nil
	case schema.Domain_CLOUD_MANAGED:
		return storage.Domain_CLOUD_MANAGED, nil
	case schema.Domain_USER_SPECIFIED:
		return storage.Domain_USER_SPECIFIED, nil
	case schema.Domain_USER_SPECIFIED_TLS_MANAGED:
		return storage.Domain_USER_SPECIFIED_TLS_MANAGED, nil
	default:
		return storage.Domain_MANAGED_UNKNOWN, fnerrors.InternalError("unknown domain managed type: %s", managed)
	}
}

// Deprecated
func addDeprecatedFields(r *storage.NetworkPlan) {
	var nonLocalManaged, nonLocalNonManaged []*filteredDomain

	domains := filterAndDedupDomains(r.IngressFragments, nil)
	for _, n := range domains {
		// Local domains need `ns dev` for port forwarding.
		if n.Domain.GetManaged() == storage.Domain_USER_SPECIFIED {
			nonLocalNonManaged = append(nonLocalNonManaged, n)
		} else if n.Domain.GetManaged() == storage.Domain_CLOUD_MANAGED || n.Domain.GetManaged() == storage.Domain_USER_SPECIFIED_TLS_MANAGED {
			nonLocalManaged = append(nonLocalManaged, n)
		}
	}

	var localDomains []*filteredDomain
	for _, n := range domains {
		if n.Domain.GetManaged() == storage.Domain_LOCAL_MANAGED {
			localDomains = append(localDomains, n)
		}
	}

	r.NonLocalManaged = renderIngressBlock(nonLocalManaged)
	r.NonLocalNonManaged = renderIngressBlock(nonLocalNonManaged)

	summary := render.NetworkPlanToSummary(r)
	services := append(summary.FocusedServices, summary.SupportServices...)
	for _, s := range services {
		endpoint := &storage.NetworkPlan_Endpoint{
			Label:         &storage.NetworkPlan_Label{Label: s.Label.Label, ServiceProto: s.Label.ServiceProto},
			Focus:         s.Focus,
			LocalPort:     s.LocalPort,
			EndpointOwner: s.PackageName,
		}
		for _, cmd := range s.AccessCmd {
			endpoint.AccessCmd = append(endpoint.AccessCmd, &storage.NetworkPlan_AccessCmd{
				Cmd: cmd.Cmd, IsManaged: cmd.IsManaged})
		}
		r.Endpoint = append(r.Endpoint, endpoint)
	}
}

// Deprecated
type filteredDomain struct {
	Domain    *storage.Domain
	Endpoints []*storage.Endpoint
}

// Deprecated
func filterAndDedupDomains(fragments []*storage.IngressFragment, filter func(*storage.Domain) bool) []*filteredDomain {
	seen := map[string]*filteredDomain{} // Map fqdn:type to schema.
	domains := []*filteredDomain{}
	for _, frag := range fragments {
		d := frag.Domain

		if d.GetFqdn() == "" {
			continue
		}

		if filter != nil && !filter(d) {
			continue
		}

		key := fmt.Sprintf("%s:%s", d.GetFqdn(), d.GetManaged())

		if _, ok := seen[key]; !ok {
			fd := &filteredDomain{Domain: d}
			domains = append(domains, fd)
			seen[key] = fd
		}

		if frag.Endpoint != nil {
			seen[key].Endpoints = append(seen[key].Endpoints, frag.Endpoint)
		}
	}

	return domains
}

// Deprecated
func renderIngressBlock(fragments []*filteredDomain) []*storage.NetworkPlan_Ingress {
	var ingresses []*storage.NetworkPlan_Ingress
	for _, n := range fragments {
		schema, portLabel, cmd, suffix := domainSchema(n.Domain, 443, n.Endpoints...)

		var owners uniquestrings.List
		for _, endpoint := range n.Endpoints {
			owners.Add(endpoint.ServerOwner)
		}

		ingresses = append(ingresses, &storage.NetworkPlan_Ingress{
			Fqdn:         n.Domain.Fqdn,
			Schema:       schema,
			PortLabel:    portLabel,
			Command:      cmd,
			Comment:      suffix,
			PackageOwner: owners.Strings(),
		})
	}
	return ingresses
}

// Deprecated
func domainSchema(domain *storage.Domain, localPort uint, endpoints ...*storage.Endpoint) (string, string, string, string) {
	var protocols uniquestrings.List
	for _, endpoint := range endpoints {
		for _, md := range endpoint.GetServiceMetadata() {
			if md.Protocol != "" {
				protocols.Add(md.Protocol)
			}
		}
	}

	var schema, portLabel, cmd, suffix string
	switch len(protocols.Strings()) {
	case 0:
		schema, portLabel = httpSchema(domain, localPort)
	case 1:
		if protocols.Strings()[0] == "grpc" {
			schema, portLabel, cmd = grpcSchema(domain.HasCertificate, localPort)
			if !domain.HasCertificate {
				suffix = "not currently working, see #26"
			}
		} else {
			schema, portLabel = httpSchema(domain, localPort)
		}
	default:
		schema = "(multiple: " + strings.Join(protocols.Strings(), ", ") + ") "
	}

	return schema, portLabel, cmd, suffix
}

// Deprecated
func grpcSchema(tls bool, port uint) (string, string, string) {
	if tls {
		return "ns tools grpcurl", fmt.Sprintf(":%d", port), " list"
	}
	return "ns tools grpcurl -plaintext", fmt.Sprintf(":%d", port), " list"
}

// Deprecated
func httpSchema(d *storage.Domain, port uint) (string, string) {
	if d.HasCertificate || port == 443 {
		return "https://", checkPort(port, 443)
	}
	return "http://", checkPort(port, 80)
}

// Deprecated
func checkPort(port, expected uint) string {
	if port == expected {
		return ""
	}
	return fmt.Sprintf(":%d", port)
}
