// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package deploy

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/morikuni/aec"
	"github.com/muesli/reflow/padding"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/uniquestrings"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/tasks"
)

type PortFwd struct {
	Endpoint  *schema.Endpoint
	LocalPort uint
}

func RenderPortsAndIngresses(checkmark bool, out io.Writer, localHostname string, stack *schema.Stack, focus []*schema.Server, portFwds []*PortFwd, ingressDomains []*runtime.FilteredDomain, ingress []*schema.IngressFragment) {
	var longest uint
	for _, p := range portFwds {
		if !isIngress(p.Endpoint) {
			label := makeServiceLabel(stack, p.Endpoint)
			if l := uint(len(label)); l > longest {
				longest = l
			}
		}
	}

	var longestUrl, ingressFwdCount, internalCount uint
	urls := map[int]string{}
	for k, p := range portFwds {
		if isInternal(p.Endpoint) {
			internalCount++
			continue
		}

		if isIngress(p.Endpoint) {
			ingressFwdCount++
			continue
		}

		if p.Endpoint.Port == nil {
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
			schema, portLabel := grpcSchema(false, p.LocalPort)
			url = fmt.Sprintf("%s%s%s", schema, localHostname, portLabel)
		} else if protocols.Has("http") {
			url = fmt.Sprintf("http://%s:%d", localHostname, p.LocalPort)
		} else {
			url = fmt.Sprintf("%s:%d --> %d", localHostname, p.LocalPort, p.Endpoint.Port.ContainerPort)
		}

		urls[k] = url
		if l := uint(len(url)); l > longestUrl {
			longestUrl = l
		}
	}

	if localHostname == "" {
		fmt.Fprintf(out, " Services deployed:\n\n")
	} else {
		fmt.Fprintf(out, " Services forwarded to %s", aec.Italic.Apply("localhost"))
		if internalCount > 0 {
			fmt.Fprintf(out, " (+%d internal)", internalCount)
		}
		fmt.Fprintf(out, ":\n\n")
	}

	for k, p := range portFwds {
		if isIngress(p.Endpoint) || isInternal(p.Endpoint) {
			continue
		}

		var protocols uniquestrings.List
		label := makeServiceLabel(stack, p.Endpoint)

		for _, md := range p.Endpoint.ServiceMetadata {
			protocols.Add(md.Protocol)
		}

		isFocus := isFocusEndpoint(focus, p.Endpoint)
		if isFocus {
			label = colors.Bold(label)
		} else {
			label = tasks.ColorFade.Apply(label)
		}

		url := urls[k]
		if !isFocus {
			url = tasks.ColorFade.Apply(url)
		}

		if isFocusEndpoint(focus, p.Endpoint) {
			if k > 0 && !isFocusEndpoint(focus, portFwds[k-1].Endpoint) {
				fmt.Fprintln(out)
			}
		}

		fmt.Fprintf(out, " %s%s  %s%s\n", checkLabel(checkmark, isFocus, p.LocalPort), padding.String(label, longest), padding.String(url, longestUrl+2), comment(p.Endpoint.EndpointOwner))
	}

	var nonLocalManaged, nonLocalNonManaged []*schema.IngressFragment
	for _, n := range ingress {
		// Local domains need `fn dev` for port forwarding.
		if n.GetDomain().GetManaged() == schema.Domain_USER_SPECIFIED {
			nonLocalNonManaged = append(nonLocalNonManaged, n)
		} else if n.GetDomain().GetManaged() == schema.Domain_CLOUD_MANAGED || n.GetDomain().GetManaged() == schema.Domain_USER_SPECIFIED_TLS_MANAGED {
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
			renderIngress(checkmark, out, localDomains, p.LocalPort, "Ingress endpoints forwarded to your workstation")
		}
	}

	if ingressFwdCount == 0 && len(nonLocalManaged) > 0 {
		renderIngressBlock(out, "Ingress configured", nonLocalManaged)
	}

	if ingressFwdCount == 0 && len(nonLocalNonManaged) > 0 {
		renderIngressBlock(out, "Ingress configured, but not managed", nonLocalNonManaged)
	}
}

func renderIngressBlock(out io.Writer, label string, fragments []*schema.IngressFragment) {
	fmt.Fprintf(out, "\n %s:\n\n", label)

	labels := make([]string, len(fragments))
	suffixes := make([]string, len(fragments))

	var longestLabel uint
	for k, n := range fragments {
		schema, portLabel, suffix := domainSchema(n.Domain, 443, n.Endpoint)
		labels[k] = fmt.Sprintf("%s%s%s", schema, n.Domain.Fqdn, portLabel)
		suffixes[k] = suffix

		if x := uint(len(labels[k])); x > longestLabel {
			longestLabel = x
		}
	}

	for k, n := range fragments {
		fmt.Fprintf(out, " %s%s %s%s\n", checkbox(true, false), padding.String(labels[k], longestLabel), comment(n.Owner), suffixes[k])
	}
}

func renderIngress(checkmark bool, out io.Writer, ingressDomains []*runtime.FilteredDomain, localPort uint, label string) {
	if len(ingressDomains) == 0 {
		return
	}

	fmt.Fprintf(out, "\n %s:\n\n", label)

	for _, fd := range ingressDomains {
		var protocols uniquestrings.List
		for _, endpoint := range fd.Endpoints {
			for _, md := range endpoint.ServiceMetadata {
				if md.Protocol != "" {
					protocols.Add(md.Protocol)
				}
			}
		}

		schema, portLabel, suffix := domainSchema(fd.Domain, localPort, fd.Endpoints...)

		fmt.Fprintf(out, " %s%s%s%s%s\n", checkLabel(checkmark, true, localPort), schema, fd.Domain.Fqdn, portLabel, suffix)
	}
}

func domainSchema(domain *schema.Domain, localPort uint, endpoints ...*schema.Endpoint) (string, string, string) {
	var protocols uniquestrings.List
	for _, endpoint := range endpoints {
		for _, md := range endpoint.GetServiceMetadata() {
			if md.Protocol != "" {
				protocols.Add(md.Protocol)
			}
		}
	}

	var schema, portLabel, suffix string
	switch len(protocols.Strings()) {
	case 0:
		schema, portLabel = httpSchema(domain, localPort)
	case 1:
		if protocols.Strings()[0] == "grpc" {
			schema, portLabel = grpcSchema(domain.Certificate != nil, localPort)
			if domain.Certificate == nil {
				suffix = tasks.ColorFade.Apply(" # not currently working, see #26")
			}
		} else {
			schema, portLabel = httpSchema(domain, localPort)
		}
	default:
		schema = "(multiple: " + strings.Join(protocols.Strings(), ", ") + ") "
	}

	return schema, portLabel, suffix
}

func checkLabel(b, isFocus bool, port uint) string {
	return checkbox(!b || port > 0, !isFocus)
}

func checkbox(on, dimmed bool) string {
	x := " [ ] "
	if on {
		x = " [✓] "
	}
	if dimmed {
		return tasks.ColorFade.Apply(x)
	}
	return x
}

func grpcSchema(tls bool, port uint) (string, string) {
	if tls {
		return "grpcurl ", checkPort(port, 443)
	}
	return "grpcurl -plaintext ", checkPort(port, 80)
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

func comment(str string) string {
	if str == "" {
		return ""
	}
	return tasks.ColorFade.Apply("# " + str)
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

func makeServiceLabel(stack *schema.Stack, endpoint *schema.Endpoint) string {
	entry := stack.GetServer(schema.PackageName(endpoint.EndpointOwner))
	if entry != nil {
		if endpoint.ServiceName == runtime.GrpcGatewayServiceName {
			return "gRPC gateway"
		}

		return fmt.Sprintf("%s/%s", entry.Server.Name, endpoint.ServiceName)
	}

	for _, md := range endpoint.ServiceMetadata {
		if md.Protocol == schema.GrpcProtocol {
			return compressProtoTypename(md.Kind)
		}
	}

	return endpoint.ServiceName
}

func compressProtoTypename(t string) string {
	if len(t) < 40 {
		return t
	}
	parts := strings.Split(t, ".")
	for k := 0; k < len(parts)-1; k++ {
		parts[k] = string(parts[k][0])
	}
	return strings.Join(parts, ".")
}
