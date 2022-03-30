// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package ingress

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"google.golang.org/protobuf/types/known/anypb"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	applynetworkingv1 "k8s.io/client-go/applyconfigurations/networking/v1"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/runtime/kubernetes/networking/ingress/nginx"
	"namespacelabs.dev/foundation/schema"
)

func EnsureStack(ctx context.Context) ([]*schema.Definition, error) {
	// XXX make this configurable.
	return nginx.Ensure(ctx)
}

type MapAddress struct {
	FQDN    string
	Ingress IngressRef
}

type IngressRef struct {
	Namespace, Name string
}

func Ensure(ctx context.Context, ns string, env *schema.Environment, srv *schema.Server, fragments []*schema.IngressFragment, certSecrets map[string]string) ([]kubedef.Apply, []MapAddress, error) {
	var applies []kubedef.Apply

	// Since we built the Cert list from a map, it's order is non-deterministic.
	sort.Slice(applies, func(i, j int) bool {
		return strings.Compare(applies[i].Name, applies[j].Name) < 0
	})

	groups := groupByName(fragments)

	var managed []MapAddress
	for _, g := range groups {
		apply, m, err := generateForSrv(ns, env, srv, g[0].Name, g, certSecrets)
		if err != nil {
			return nil, nil, err
		}

		applies = append(applies, kubedef.Apply{
			Description: fmt.Sprintf("Ingress %s", g[0].Name),
			Resource:    "ingresses",
			Namespace:   ns,
			Name:        g[0].Name,
			Body:        apply,
		})

		managed = append(managed, m...)
	}

	return applies, managed, nil
}

func MakeCertificateSecrets(ns string, fragments []*schema.IngressFragment) (map[string]string, []kubedef.Apply) {
	var applies []kubedef.Apply

	domainCerts := map[string]*schema.Domain{}
	for _, frag := range fragments {
		if frag.Domain != nil && frag.Domain.Fqdn != "" && frag.Domain.Certificate != nil {
			// XXX check they're consistent.
			domainCerts[frag.Domain.Fqdn] = frag.Domain
		}
	}

	certSecrets := map[string]string{} // Map fqdn->secret name.
	for _, domain := range domainCerts {
		name := fmt.Sprintf("tls-%s", strings.ReplaceAll(domain.Fqdn, ".", "-"))
		certSecrets[domain.Fqdn] = name
		applies = append(applies, kubedef.Apply{
			Description: fmt.Sprintf("Certificate for %s", domain.Fqdn),
			Resource:    "secrets",
			Namespace:   ns,
			Name:        name,
			Body: applycorev1.
				Secret(name, ns).
				WithType(corev1.SecretTypeTLS).
				WithLabels(kubedef.ManagedBy()).
				WithData(map[string][]byte{
					"tls.key": domain.Certificate.PrivateKey,
					"tls.crt": domain.Certificate.CertificateBundle,
				}),
		})
	}

	// Since we built the Cert list from a map, it's order is non-deterministic.
	sort.Slice(applies, func(i, j int) bool {
		return strings.Compare(applies[i].Name, applies[j].Name) < 0
	})

	return certSecrets, applies
}

func groupByName(ngs []*schema.IngressFragment) [][]*schema.IngressFragment {
	sort.Slice(ngs, func(i, j int) bool {
		return strings.Compare(ngs[i].Name, ngs[j].Name) < 0
	})

	var groups [][]*schema.IngressFragment

	var currentName string
	var g []*schema.IngressFragment
	for _, ng := range ngs {
		if ng.Name != currentName {
			if len(g) > 0 {
				groups = append(groups, g)
			}
			g = nil
			currentName = ng.Name
		}

		g = append(g, ng)
	}
	if len(g) > 0 {
		groups = append(groups, g)
	}

	return groups
}

func generateForSrv(ns string, env *schema.Environment, srv *schema.Server, name string, ngs []*schema.IngressFragment, certSecrets map[string]string) (*applynetworkingv1.IngressApplyConfiguration, []MapAddress, error) {
	backendProtocol := "http"

	var grpcCount, nonGrpcCount int

	spec := applynetworkingv1.IngressSpec()

	var managedType schema.Domain_ManagedType
	for _, ng := range ngs {
		if ng.GetDomain().GetManaged() == schema.Domain_MANAGED_UNKNOWN {
			continue
		}
		if managedType != schema.Domain_MANAGED_UNKNOWN && managedType != ng.GetDomain().GetManaged() {
			return nil, nil, fnerrors.InternalError("inconsistent domain definition, %q vs %q", managedType, ng.GetDomain().GetManaged())
		}
		managedType = ng.GetDomain().GetManaged()
	}

	var tlsCount int
	var managed []MapAddress
	var extensions []*anypb.Any
	for _, ng := range ngs {
		extensions = append(extensions, ng.Extension...)

		var paths []*applynetworkingv1.HTTPIngressPathApplyConfiguration
		for _, p := range ng.HttpPath {
			if p.Kind == "grpc" {
				grpcCount++
			} else {
				nonGrpcCount++
			}

			if p.Port == nil {
				return nil, nil, fnerrors.InternalError("%s: ingress definition without port", filepath.Join(p.Path, p.Service))
			}

			// XXX validate ports.
			paths = append(paths, applynetworkingv1.HTTPIngressPath().
				WithPath(p.Path).
				WithPathType(netv1.PathTypePrefix).
				WithBackend(
					applynetworkingv1.IngressBackend().WithService(
						applynetworkingv1.IngressServiceBackend().WithName(p.Service).WithPort(
							applynetworkingv1.ServiceBackendPort().WithNumber(p.Port.ContainerPort)))))
		}

		spec = spec.WithRules(applynetworkingv1.IngressRule().WithHost(ng.Domain.Fqdn).WithHTTP(
			applynetworkingv1.HTTPIngressRuleValue().WithPaths(
				paths...)))

		if tlsSecret, ok := certSecrets[ng.Domain.Fqdn]; ok {
			spec = spec.WithTLS(applynetworkingv1.IngressTLS().WithHosts(ng.Domain.Fqdn).WithSecretName(tlsSecret))
			tlsCount++
		}

		if ng.Domain.Managed == schema.Domain_CLOUD_MANAGED {
			managed = append(managed, MapAddress{
				FQDN:    ng.Domain.Fqdn,
				Ingress: IngressRef{ns, name},
			})
		}
	}

	if grpcCount > 0 && nonGrpcCount > 0 {
		return nil, nil, fnerrors.InternalError("can't mix grpc and non-grpc backends in the same ingress")
	}

	if grpcCount > 0 {
		// XXX grpc vs grpcs
		backendProtocol = "grpc"
	}

	// XXX make nginx configurable.
	annotations, err := nginx.IngressAnnotations(tlsCount > 0, backendProtocol, extensions)
	if err != nil {
		return nil, nil, err
	}

	ingress := applynetworkingv1.Ingress(name, ns).
		WithLabels(kubedef.MakeLabels(env, srv)).
		WithAnnotations(annotations).
		WithSpec(spec)

	return ingress, managed, nil
}

func Delete(ns string, stack []provision.Server) ([]*schema.Definition, error) {
	var defs []*schema.Definition

	for _, srv := range stack {
		op := kubedef.DeleteList{
			Description: "Ingresses",
			Resource:    "ingresses",
			Namespace:   ns,
			Selector:    kubedef.SelectById(srv.Proto()),
		}

		if def, err := op.ToDefinition(srv.PackageName()); err != nil {
			return nil, err
		} else {
			defs = append(defs, def)
		}
	}

	return defs, nil
}
