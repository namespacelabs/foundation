// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package deploy

import (
	"fmt"
	"sort"
	"strings"

	"namespacelabs.dev/foundation/internal/uniquestrings"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/schema/storage"
)

type PortFwd struct {
	Endpoint  *schema.Endpoint
	LocalPort uint
}

func RenderPortsAndIngresses(localHostname string, stack *schema.Stack, focus []*schema.Server, portFwds []*PortFwd, ingressDomains []*runtime.FilteredDomain, ingress []*schema.IngressFragment) *storage.NetworkPlan {
	r := &storage.NetworkPlan{}

	for _, p := range portFwds {
		if isInternal(p.Endpoint) {
			r.InternalCount++
			continue
		}

		if isIngress(p.Endpoint) || p.Endpoint.Port == nil {
			continue
		}

		var protocols uniquestrings.List
		for _, md := range p.Endpoint.ServiceMetadata {
			protocols.Add(md.Protocol)
		}

		var url string
		if p.LocalPort == 0 {
			url = fmt.Sprintf("container port %d", p.Endpoint.Port.ContainerPort)

			if protocols.Has("grpc") {
				url = "grpc " + url
			} else if protocols.Has("http") {
				url = "http " + url
			}
		} else if protocols.Has("grpc") {
			// grpc endpoints also do http, but we only expose one kind of service here.
			schema, portLabel, cmd := grpcSchema(false, p.LocalPort)
			url = fmt.Sprintf("%s%s%s%s", schema, localHostname, portLabel, cmd)
		} else if protocols.Has("http") {
			url = fmt.Sprintf("http://%s:%d", localHostname, p.LocalPort)
		} else {
			url = fmt.Sprintf("%s:%d --> %d", localHostname, p.LocalPort, p.Endpoint.Port.ContainerPort)
		}

		r.Endpoint = append(r.Endpoint, &storage.NetworkPlan_Endpoint{
			Focus:         isFocusEndpoint(focus, p.Endpoint),
			Label:         makeServiceLabel(stack, p.Endpoint),
			Url:           url,
			LocalPort:     uint32(p.LocalPort),
			EndpointOwner: p.Endpoint.EndpointOwner, // Comment
		})
	}

	var nonLocalManaged, nonLocalNonManaged []*runtime.FilteredDomain

	for _, n := range runtime.FilterAndDedupDomains(ingress, nil) {
		// Local domains need `ns dev` for port forwarding.
		if n.Domain.GetManaged() == schema.Domain_USER_SPECIFIED {
			nonLocalNonManaged = append(nonLocalNonManaged, n)
		} else if n.Domain.GetManaged() == schema.Domain_CLOUD_MANAGED || n.Domain.GetManaged() == schema.Domain_USER_SPECIFIED_TLS_MANAGED {
			nonLocalManaged = append(nonLocalManaged, n)
		}
	}

	var localDomains, cloudDomains []*runtime.FilteredDomain
	for _, n := range ingressDomains {
		if n.Domain.GetManaged() == schema.Domain_LOCAL_MANAGED {
			localDomains = append(localDomains, n)
		} else if n.Domain.GetManaged() == schema.Domain_CLOUD_MANAGED {
			cloudDomains = append(cloudDomains, n)
		}
	}

	for _, p := range portFwds {
		if isIngress(p.Endpoint) {
			r.Ingress = append(r.Ingress, renderIngress(localDomains, p.LocalPort)...)
		}
	}

	r.NonLocalManaged = renderIngressBlock(nonLocalManaged)
	r.NonLocalNonManaged = renderIngressBlock(nonLocalNonManaged)

	return r
}

func renderIngressBlock(fragments []*runtime.FilteredDomain) []*storage.NetworkPlan_Ingress {
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

func renderIngress(ingressDomains []*runtime.FilteredDomain, localPort uint) []*storage.NetworkPlan_Ingress {
	if len(ingressDomains) == 0 {
		return nil
	}

	var ingresses []*storage.NetworkPlan_Ingress
	for _, fd := range ingressDomains {
		schema, portLabel, cmd, suffix := domainSchema(fd.Domain, localPort, fd.Endpoints...)

		ingresses = append(ingresses, &storage.NetworkPlan_Ingress{
			LocalPort: uint32(localPort),
			Schema:    schema,
			Fqdn:      fd.Domain.Fqdn,
			PortLabel: portLabel,
			Command:   cmd,
			Comment:   suffix,
		})
	}
	return ingresses
}

func domainSchema(domain *schema.Domain, localPort uint, endpoints ...*schema.Endpoint) (string, string, string, string) {
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
			schema, portLabel, cmd = grpcSchema(domain.Certificate != nil, localPort)
			if domain.Certificate == nil {
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

func grpcSchema(tls bool, port uint) (string, string, string) {
	if tls {
		return "grpcurl ", fmt.Sprintf(":%d", port), " list"
	}
	return "grpcurl -plaintext ", fmt.Sprintf(":%d", port), " list"
}

func httpSchema(d *schema.Domain, port uint) (string, string) {
	if d.Certificate != nil || port == 443 {
		return "https://", checkPort(port, 443)
	}
	return "http://", checkPort(port, 80)
}

func checkPort(port, expected uint) string {
	if port == expected {
		return ""
	}
	return fmt.Sprintf(":%d", port)
}

func SortIngresses(ingress []*schema.IngressFragment) {
	sort.Slice(ingress, func(i, j int) bool {
		a, b := ingress[i].Domain.Fqdn, ingress[j].Domain.Fqdn

		ap := strings.Split(a, ".")
		bp := strings.Split(b, ".")

		if len(ap) > 3 {
			ap = ap[len(ap)-3:]
		}
		if len(bp) > 3 {
			bp = bp[len(bp)-3:]
		}

		az := strings.Join(ap, ".")
		bz := strings.Join(bp, ".")

		if az == bz {
			return strings.Compare(a, b) < 0
		}

		// XXX move these constants out.

		// Sort generated domain names first.
		if ingress[i].Domain.GetManaged() == schema.Domain_LOCAL_MANAGED || ingress[i].Domain.GetManaged() == schema.Domain_CLOUD_MANAGED {
			return true
		} else if ingress[j].Domain.GetManaged() == schema.Domain_LOCAL_MANAGED || ingress[j].Domain.GetManaged() == schema.Domain_CLOUD_MANAGED {
			return false
		}

		return strings.Compare(az, bz) < 0
	})
}

func SortPorts(portFwds []*PortFwd, focus []*schema.Server) {
	sort.Slice(portFwds, func(i, j int) bool {
		a, b := portFwds[i], portFwds[j]

		if isIngress(b.Endpoint) {
			if isIngress(a.Endpoint) {
				return a.LocalPort < b.LocalPort
			}

			return true
		} else if isIngress(a.Endpoint) {
			return false
		} else {
			if isFocusEndpoint(focus, a.Endpoint) {
				if !isFocusEndpoint(focus, b.Endpoint) {
					return false
				}
			} else if isFocusEndpoint(focus, b.Endpoint) {
				return true
			}

			return strings.Compare(a.Endpoint.ServiceName, b.Endpoint.ServiceName) < 0
		}
	})
}

func isFocusEndpoint(focus []*schema.Server, endpoint *schema.Endpoint) bool {
	for _, s := range focus {
		if s.GetPackageName() == endpoint.ServerOwner {
			return true
		}
	}

	return false
}

func isIngress(endpoint *schema.Endpoint) bool {
	return endpoint.EndpointOwner == "" && endpoint.ServiceName == runtime.IngressServiceName
}

func isInternal(endpoint *schema.Endpoint) bool {
	for _, md := range endpoint.ServiceMetadata {
		if md.Kind == runtime.ManualInternalService {
			return true
		}
	}

	return false
}

func makeServiceLabel(stack *schema.Stack, endpoint *schema.Endpoint) *storage.NetworkPlan_Label {
	entry := stack.GetServer(schema.PackageName(endpoint.EndpointOwner))
	if entry != nil {
		if endpoint.ServiceName == runtime.GrpcGatewayServiceName {
			return &storage.NetworkPlan_Label{Label: "gRPC gateway"}
		}

		if endpoint.ServiceLabel != "" {
			return &storage.NetworkPlan_Label{Label: endpoint.ServiceLabel}
		}

		return &storage.NetworkPlan_Label{Label: fmt.Sprintf("%s/%s", entry.Server.Name, endpoint.ServiceName)}
	}

	for _, md := range endpoint.ServiceMetadata {
		if md.Protocol == schema.GrpcProtocol {
			return &storage.NetworkPlan_Label{ServiceProto: md.Kind}
		}
	}

	if endpoint.ServiceLabel != "" {
		return &storage.NetworkPlan_Label{Label: endpoint.ServiceLabel}
	}

	return &storage.NetworkPlan_Label{Label: endpoint.ServiceName}
}
