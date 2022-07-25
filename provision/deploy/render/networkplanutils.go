// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

// Contains function to convert NetworkPlan to messages convenient for UI (terminal, web).

package render

import (
	"fmt"
	"net/url"
	"sort"
	"strings"

	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/schema/storage"
)

func NetworkPlanToSummary(plan *storage.NetworkPlan) *NetworkPlanSummary {
	endpoints := plan.Endpoints
	sortPorts(endpoints, plan.FocusedServerPackages)
	ingressFragments := plan.IngressFragments
	sortIngresses(ingressFragments)

	s := &NetworkPlanSummary{
		LocalHostname:   plan.LocalHostname,
		FocusedServices: []*NetworkPlanSummary_Service{},
		SupportServices: []*NetworkPlanSummary_Service{},
	}

	localIngressPort := uint32(0)
	for _, p := range endpoints {
		if isIngress(p) {
			localIngressPort = p.LocalPort
			break
		}
	}

	for _, p := range endpoints {
		if isInternal(p) {
			continue
		}

		if isIngress(p) || p.Port == nil {
			continue
		}

		protocolToKind := map[string]string{}
		for _, md := range p.ServiceMetadata {
			protocolToKind[md.Protocol] = md.Kind
		}

		isFocus := isFocusEndpoint(plan.FocusedServerPackages, p)

		endpoint := &NetworkPlanSummary_Service{
			Focus:       isFocus,
			Label:       makeServiceLabel(p),
			AccessCmd:   []*NetworkPlanSummary_Service_AccessCmd{},
			LocalPort:   uint32(p.LocalPort),
			PackageName: p.EndpointOwner,
		}

		var httpAccessCmds []*NetworkPlanSummary_Service_AccessCmd
		var grpcAccessCmds []*NetworkPlanSummary_Service_AccessCmd

		for _, ingress := range ingressFragments {
			isManaged := ingress.Domain.GetManaged() != storage.Domain_USER_SPECIFIED &&
				ingress.Domain.GetManaged() != storage.Domain_USER_SPECIFIED_TLS_MANAGED

			for _, httpPath := range ingress.HttpPath {
				url := httpUrl(ingress.Domain, localIngressPort, httpPath.Path)

				// http service
				if ingress.Owner == p.EndpointOwner &&
					httpPath.Port.ContainerPort == p.Port.ContainerPort {
					httpAccessCmds = append(httpAccessCmds, &NetworkPlanSummary_Service_AccessCmd{
						Cmd:       url,
						IsManaged: isManaged,
					})
				}

				// grpc<->http transcoding
				if ingress.Endpoint != nil &&
					ingress.Endpoint.ServiceName == p.ServiceName &&
					httpPath.Owner == p.EndpointOwner {
					endpoint.AccessCmd = append(endpoint.AccessCmd, &NetworkPlanSummary_Service_AccessCmd{
						Cmd:       fmt.Sprintf("curl -X POST %s<METHOD>", url),
						IsManaged: isManaged,
					})
				}
			}

			// If LocalPort != 0 this means we are running local dev and
			// grpc does not work via ingress because of #26.
			if p.LocalPort == 0 &&
				ingress.Endpoint != nil &&
				ingress.Endpoint.ServiceName == p.ServiceName &&
				ingress.Endpoint.EndpointOwner == p.EndpointOwner {
				for _, grpcService := range ingress.GrpcService {
					for _, svc := range p.ServiceMetadata {
						if svc.Kind == grpcService.GrpcService {
							grpcAccessCmds = append(grpcAccessCmds, &NetworkPlanSummary_Service_AccessCmd{
								Cmd:       grpcAccessCmd(ingress.Domain.HasCertificate, 443, ingress.Domain.Fqdn, svc.Kind),
								IsManaged: isManaged,
							})
						}
					}
				}
			}
		}

		if kind, ok := protocolToKind["grpc"]; ok && len(grpcAccessCmds) == 0 {
			if p.LocalPort == 0 {
				grpcAccessCmds = append(grpcAccessCmds, &NetworkPlanSummary_Service_AccessCmd{
					Cmd: fmt.Sprintf("private: container port %d gprc", p.Port.ContainerPort), IsManaged: true})
			} else {
				grpcAccessCmds = append(grpcAccessCmds, &NetworkPlanSummary_Service_AccessCmd{
					Cmd: grpcAccessCmd(false, p.LocalPort, plan.LocalHostname, kind), IsManaged: true})
			}
		}

		if _, ok := protocolToKind["http"]; ok && len(httpAccessCmds) == 0 {
			if p.LocalPort == 0 {
				httpAccessCmds = append(httpAccessCmds, &NetworkPlanSummary_Service_AccessCmd{
					Cmd: fmt.Sprintf("private: container port %d http", p.Port.ContainerPort), IsManaged: true})
			} else {
				httpAccessCmds = append(httpAccessCmds, &NetworkPlanSummary_Service_AccessCmd{
					Cmd: fmt.Sprintf("http://%s:%d", plan.LocalHostname, p.LocalPort), IsManaged: true})
			}
		}

		endpoint.AccessCmd = append(endpoint.AccessCmd, httpAccessCmds...)
		endpoint.AccessCmd = append(endpoint.AccessCmd, grpcAccessCmds...)

		// No http/grpc access cmds, adding a generic message.
		if len(endpoint.AccessCmd) == 0 {
			if p.LocalPort == 0 {
				endpoint.AccessCmd = []*NetworkPlanSummary_Service_AccessCmd{{Cmd: fmt.Sprintf("private: container port %d", p.Port.ContainerPort), IsManaged: true}}
			} else {
				endpoint.AccessCmd = []*NetworkPlanSummary_Service_AccessCmd{{Cmd: fmt.Sprintf("%s:%d --> %d", plan.LocalHostname, p.LocalPort, p.Port.ContainerPort), IsManaged: true}}
			}
		}

		if isFocus {
			s.FocusedServices = append(s.FocusedServices, endpoint)
		} else {
			s.SupportServices = append(s.SupportServices, endpoint)
		}
	}

	return s
}

func grpcAccessCmd(tls bool, port uint32, hostname string, serviceName string) string {
	extraArg := ""
	if !tls {
		extraArg = " -plaintext"
	}
	return fmt.Sprintf("ns tools grpcurl%s -d '{}' %s:%d %s/<METHOD>",
		extraArg, hostname, port, serviceName)
}

func isIngress(endpoint *storage.Endpoint) bool {
	return endpoint != nil && endpoint.EndpointOwner == "" && endpoint.ServiceName == runtime.IngressServiceName
}

func isInternal(endpoint *storage.Endpoint) bool {
	for _, md := range endpoint.ServiceMetadata {
		if md.Kind == runtime.ManualInternalService {
			return true
		}
	}

	return false
}

func isFocusEndpoint(focusedPackages []string, endpoint *storage.Endpoint) bool {
	if endpoint == nil {
		return false
	}

	for _, s := range focusedPackages {
		if s == endpoint.ServerOwner {
			return true
		}
	}

	return false
}

func makeServiceLabel(endpoint *storage.Endpoint) *Label {
	// This is depends on a convention.
	// TODO: uplift this to the schema.
	if endpoint.ServiceName == "http" {
		return &Label{Label: endpoint.ServerOwner}
	}

	if endpoint.ServerName != "" {
		if endpoint.ServiceName == runtime.GrpcGatewayServiceName {
			return &Label{Label: "gRPC gateway"}
		}

		if endpoint.ServiceLabel != "" {
			return &Label{Label: endpoint.ServiceLabel}
		}

		return &Label{Label: fmt.Sprintf("%s/%s", endpoint.ServerName, endpoint.ServiceName)}
	}

	for _, md := range endpoint.ServiceMetadata {
		if md.Protocol == schema.GrpcProtocol {
			return &Label{ServiceProto: md.Kind}
		}
	}

	if endpoint.ServiceLabel != "" {
		return &Label{Label: endpoint.ServiceLabel}
	}

	return &Label{Label: endpoint.ServiceName}
}

func httpUrl(domain *storage.Domain, localIngressPort uint32, path string) string {
	var ingressPort uint32
	if domain.GetManaged() == storage.Domain_USER_SPECIFIED ||
		domain.GetManaged() == storage.Domain_CLOUD_MANAGED ||
		domain.GetManaged() == storage.Domain_USER_SPECIFIED_TLS_MANAGED {
		ingressPort = 443
	} else {
		ingressPort = localIngressPort
	}

	// Using URL for merging the base URL and the path.
	var url url.URL
	var port string
	if domain.HasCertificate || ingressPort == 443 {
		url.Scheme = "https"
		port = checkPort(ingressPort, 443)
	} else {
		url.Scheme = "http"
		port = checkPort(ingressPort, 80)
	}
	url.Host = fmt.Sprintf("%s%s", domain.Fqdn, port)
	url.Path = path

	return url.String()
}

func checkPort(port, expected uint32) string {
	if port == expected {
		return ""
	}
	return fmt.Sprintf(":%d", port)
}

func sortIngresses(ingress []*storage.IngressFragment) {
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
		if ingress[i].Domain.GetManaged() == storage.Domain_LOCAL_MANAGED || ingress[i].Domain.GetManaged() == storage.Domain_CLOUD_MANAGED {
			return true
		} else if ingress[j].Domain.GetManaged() == storage.Domain_LOCAL_MANAGED || ingress[j].Domain.GetManaged() == storage.Domain_CLOUD_MANAGED {
			return false
		}

		return strings.Compare(az, bz) < 0
	})
}

func sortPorts(portFwds []*storage.Endpoint, focusedPackages []string) {
	sort.Slice(portFwds, func(i, j int) bool {
		a, b := portFwds[i], portFwds[j]

		if isIngress(b) {
			if isIngress(a) {
				return a.LocalPort < b.LocalPort
			}

			return true
		} else if isIngress(a) {
			return false
		} else {
			if isFocusEndpoint(focusedPackages, a) {
				if !isFocusEndpoint(focusedPackages, b) {
					return false
				}
			} else if isFocusEndpoint(focusedPackages, b) {
				return true
			}

			return strings.Compare(a.ServiceName, b.ServiceName) < 0
		}
	})
}
